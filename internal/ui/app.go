package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/term"

	"msxfront/internal/db"
	"msxfront/internal/filehunter"
	"msxfront/internal/openmsx"
)

// ── Color palette ──────────────────────────────────────────────────────────
const (
	colorTitle  = tcell.ColorAqua
	colorBorder = tcell.ColorDarkCyan
	colorHighBG = tcell.ColorDarkBlue
)

// Tab identifiers
const (
	tabControl = iota
	tabBrowser
	tabFavorites
	tabHistory
	tabLogs
)

// App is the main TUI application.
type App struct {
	tv       *tview.Application
	pages    *tview.Pages
	database *db.DB
	client   *openmsx.Client
	fhClient *filehunter.Client

	// Panels
	header     *tview.TextView
	statusBar  *tview.TextView
	tabs       []*tview.TextView
	currentTab int

	// Control panel
	controlPanel *tview.Flex
	cmdInput     *tview.InputField
	cmdOutput    *tview.TextView
	ledStatus    *tview.TextView
	connInfo     *tview.TextView // kept as field so we can refresh after connect
	quickBtns    *tview.Flex

	// File browser panel
	browserPanel *tview.Flex
	browserList  *tview.List
	browserPath  *tview.TextView
	browserInfo  *tview.TextView
	searchInput  *tview.InputField
	browserStack []string

	// Favorites panel
	favPanel *tview.Flex
	favList  *tview.List

	// History panel
	histPanel *tview.Flex
	histList  *tview.List

	// Logs panel
	logPanel *tview.Flex
	logView  *tview.TextView

	// State
	connected   bool
	currentAddr string
	leds        map[string]string
	fhEntries   []filehunter.Entry
	favEntries  []db.Favorite
}

// NewApp creates and wires the TUI.
func NewApp(database *db.DB) *App {
	a := &App{
		tv:           tview.NewApplication(),
		database:     database,
		fhClient:     filehunter.New(),
		leds:         make(map[string]string),
		browserStack: []string{""},
	}
	a.build()
	return a
}

// Run starts the TUI event loop.
func (a *App) Run() error {
	return a.tv.Run()
}

// ── Build ─────────────────────────────────────────────────────────────────

// termSize returns current terminal columns and rows.
func termSize() (cols, rows int) {
	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || cols == 0 {
		return 80, 24 // safe default
	}
	return cols, rows
}

// layoutMode classifies the terminal into three sizes.
// compact  : rows < 28  (e.g. default Windows Terminal 24 rows)
// normal   : 28 ≤ rows < 40
// spacious : rows ≥ 40
type layoutMode int

const (
	layoutCompact layoutMode = iota
	layoutNormal
	layoutSpacious
)

func detectLayout() layoutMode {
	_, rows := termSize()
	switch {
	case rows < 28:
		return layoutCompact
	case rows < 40:
		return layoutNormal
	default:
		return layoutSpacious
	}
}

func (a *App) build() {
	mode := detectLayout()

	a.buildHeader(mode)
	a.buildStatusBar()
	a.buildControlPanel(mode)
	a.buildBrowserPanel()
	a.buildFavoritesPanel()
	a.buildHistoryPanel()
	a.buildLogPanel()

	a.pages = tview.NewPages()
	a.pages.AddPage("control", a.controlPanel, true, true)
	a.pages.AddPage("browser", a.browserPanel, true, false)
	a.pages.AddPage("favorites", a.favPanel, true, false)
	a.pages.AddPage("history", a.histPanel, true, false)
	a.pages.AddPage("logs", a.logPanel, true, false)

	// Header height: 1 line in compact, 3 in normal/spacious
	headerH := 3
	if mode == layoutCompact {
		headerH = 1
	}

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, headerH, 0, false).
		AddItem(a.buildTabBar(), 1, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.statusBar, 1, 0, false)

	a.tv.SetRoot(root, true).EnableMouse(true)
	a.tv.SetInputCapture(a.globalKeys)

	_, rows := termSize()
	if rows < 24 {
		a.setStatus(fmt.Sprintf(
			"[yellow]⚠ Terminal pequeno (%d linhas). Recomendado: 28+ linhas. [-]", rows))
	} else {
		a.setStatus("Pronto. Pressione F2 para conectar ao openMSX.")
	}
	a.switchTab(tabControl)
}

func (a *App) buildHeader(mode layoutMode) {
	a.header = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	switch mode {
	case layoutCompact:
		// Single line — fits in 1 row
		a.header.SetText("[aqua::b]MSXFront[-] [darkcyan]— openMSX Controller + FileHunter[-]")
	case layoutNormal:
		// Two lines — compact ASCII + subtitle
		a.header.SetText(
			"[aqua::b]  ███╗   ███╗███████╗██╗  ██╗    ██╗      █████╗ ██╗   ██╗███╗   ██╗  [-]\n" +
				"[darkcyan]        MSXFront — openMSX TUI Controller + FileHunter Browser[-]",
		)
	default:
		// Three lines — full ASCII art
		a.header.SetText(
			"[aqua::b] ███╗   ███╗███████╗██╗  ██╗    ██╗      █████╗ ██╗   ██╗███╗   ██╗ [-]\n" +
				"[aqua::b] ████╗ ████║██╔════╝╚██╗██╔╝    ██║     ██╔══██╗██║   ██║████╗  ██║ [-]\n" +
				"[darkcyan]       MSXFront — openMSX TUI Controller + FileHunter Browser      [-]",
		)
	}
	a.header.SetBorderColor(colorBorder)
}

func (a *App) buildTabBar() *tview.Flex {
	labels := []string{
		" [F1] Controle ",
		" [F3] Browser ",
		" [F4] Favoritos ",
		" [F5] Histórico ",
		" [F6] Logs ",
	}
	a.tabs = make([]*tview.TextView, len(labels))
	bar := tview.NewFlex().SetDirection(tview.FlexColumn)
	for i, label := range labels {
		tv := tview.NewTextView().SetDynamicColors(true).SetText("[white]" + label)
		tv.SetBackgroundColor(colorHighBG)
		a.tabs[i] = tv
		bar.AddItem(tv, 0, 1, false)
	}
	return bar
}

func (a *App) switchTab(tab int) {
	a.currentTab = tab
	names := []string{"control", "browser", "favorites", "history", "logs"}
	for i, tv := range a.tabs {
		if i == tab {
			tv.SetBackgroundColor(tcell.ColorDarkCyan)
			tv.SetTextColor(tcell.ColorWhite)
		} else {
			tv.SetBackgroundColor(colorHighBG)
			tv.SetTextColor(tcell.ColorGray)
		}
	}
	a.pages.SwitchToPage(names[tab])
}

// ── Control Panel ─────────────────────────────────────────────────────────

func (a *App) buildControlPanel(mode layoutMode) {
	// LED status area
	a.ledStatus = tview.NewTextView().
		SetDynamicColors(true).
		SetText("[gray]LEDs: aguardando conexão...")
	a.ledStatus.SetBorder(true).SetTitle(" LEDs ").SetBorderColor(colorBorder)

	// Quick buttons — in compact mode show 2 rows of 4, otherwise 1 row of 8
	a.quickBtns = tview.NewFlex().SetDirection(tview.FlexColumn)
	btns := []struct{ label, cmd string }{
		{"[green]⏻ ON[white]", "set power on"},
		{"[red]⏻ OFF[white]", "set power off"},
		{"[yellow]↺ Reset[white]", "reset"},
		{"[cyan]⏸ Pause[white]", "set pause on"},
		{"[cyan]▶ Play[white]", "set pause off"},
		{"[magenta]⏪ Rwd ON[white]", "reverse enable"},
		{"[gray]⏪ Rwd OFF[white]", "reverse disable"},
		{"[white]📸 Shot[white]", "screenshot"},
	}
	for _, b := range btns {
		btn := b
		tv := tview.NewTextView().
			SetDynamicColors(true).
			SetText(btn.label).
			SetTextAlign(tview.AlignCenter)
		tv.SetBorder(true).SetBorderColor(colorBorder)
		tv.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
			if e.Key() == tcell.KeyEnter {
				a.sendCommand(btn.cmd)
				return nil
			}
			return e
		})
		a.quickBtns.AddItem(tv, 0, 1, false)
	}

	// Command input
	a.cmdInput = tview.NewInputField().
		SetLabel("[aqua] > [-] ").
		SetLabelColor(colorTitle).
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
		SetPlaceholder("Digite um comando Tcl e pressione Enter...")
	a.cmdInput.SetBorder(true).SetTitle(" Comando Manual ").SetBorderColor(colorBorder)
	a.cmdInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			cmd := strings.TrimSpace(a.cmdInput.GetText())
			if cmd != "" {
				a.sendCommand(cmd)
				a.cmdInput.SetText("")
			}
		}
	})

	// Output area
	a.cmdOutput = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	a.cmdOutput.SetBorder(true).SetTitle(" Respostas ").SetBorderColor(colorBorder)

	// Connection info
	a.connInfo = tview.NewTextView().
		SetDynamicColors(true).
		SetText(a.connInfoText())
	a.connInfo.SetBorder(true).SetTitle(" Conexão ").SetBorderColor(colorBorder)

	// Savestates panel
	savePanel := tview.NewFlex().SetDirection(tview.FlexRow)
	saveInput := tview.NewInputField().SetLabel("Nome: ").SetPlaceholder("quicksave")
	saveInput.SetBorder(true).SetTitle(" Savestates ").SetBorderColor(colorBorder)
	savePanel.AddItem(saveInput, 3, 0, false)
	saveBtns := tview.NewFlex().SetDirection(tview.FlexColumn)
	for _, pair := range [][]string{{"Salvar", "save"}, {"Carregar", "load"}} {
		p := pair
		tv := tview.NewTextView().SetText(p[0]).SetTextAlign(tview.AlignCenter)
		tv.SetBorder(true).SetBorderColor(colorBorder)
		tv.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
			if e.Key() == tcell.KeyEnter {
				name := strings.TrimSpace(saveInput.GetText())
				if name == "" {
					name = "quicksave"
				}
				if p[1] == "save" {
					a.sendCommand("savestate " + name)
				} else {
					a.sendCommand("loadstate " + name)
				}
			}
			return e
		})
		saveBtns.AddItem(tv, 0, 1, false)
	}
	savePanel.AddItem(saveBtns, 3, 0, false)

	// Right column height adapts: compact hides savestates to save rows
	rightCol := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.connInfo, 4, 0, false).
		AddItem(a.ledStatus, 3, 0, false)
	if mode != layoutCompact {
		rightCol.AddItem(savePanel, 7, 0, false)
	}

	// Button row height: 3 always fits (border + 1 content line)
	btnH := 3

	// Main layout
	top := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.quickBtns, btnH, 0, false).
		AddItem(a.cmdInput, 3, 0, true).
		AddItem(a.cmdOutput, 0, 1, false)

	a.controlPanel = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(top, 0, 3, true).
		AddItem(rightCol, 26, 0, false)
}

func (a *App) connInfoText() string {
	_, rows := termSize()
	sizeHint := fmt.Sprintf("[gray]Terminal: %d linhas[-]", rows)
	if a.connected {
		return fmt.Sprintf("[green]● Conectado[-]\n[white]%s[-]\n[gray]Máquina: %s[-]\n%s",
			a.currentAddr, a.getMachine(), sizeHint)
	}
	return fmt.Sprintf("[red]● Desconectado[-]\n[gray]F2 para conectar[-]\n\n%s", sizeHint)
}

func (a *App) getMachine() string {
	if a.client != nil && a.client.IsConnected() {
		return a.client.GetMachine()
	}
	return "-"
}

// ── File Browser Panel ────────────────────────────────────────────────────

func (a *App) buildBrowserPanel() {
	a.browserPath = tview.NewTextView().
		SetDynamicColors(true).
		SetText("[gray]/ (raiz)[white]")
	a.browserPath.SetBorder(true).SetTitle(" Caminho ").SetBorderColor(colorBorder)

	a.searchInput = tview.NewInputField().
		SetLabel("[cyan] 🔍 [-] ").
		SetLabelColor(colorTitle).
		SetPlaceholder("Buscar nesta pasta...").
		SetFieldBackgroundColor(tcell.ColorBlack)
	a.searchInput.SetBorder(true).SetTitle(" Busca ").SetBorderColor(colorBorder)
	a.searchInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			a.browserSearch()
		}
	})

	a.browserList = tview.NewList().ShowSecondaryText(true).SetHighlightFullLine(true)
	a.browserList.SetBorder(true).SetTitle(" Arquivos (Enter=abrir, F=favorito, L=carregar) ").SetBorderColor(colorBorder)
	a.browserList.SetMainTextColor(tcell.ColorWhite)
	a.browserList.SetSecondaryTextColor(tcell.ColorGray)

	a.browserInfo = tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(true).
		SetText("[gray]Selecione um arquivo para ver detalhes.")
	a.browserInfo.SetBorder(true).SetTitle(" Detalhes ").SetBorderColor(colorBorder)

	a.browserList.SetChangedFunc(func(idx int, main, sec string, r rune) {
		if idx >= 0 && idx < len(a.fhEntries) {
			e := a.fhEntries[idx]
			a.browserInfo.SetText(fmt.Sprintf(
				"[aqua]Nome:[-] %s\n[aqua]Tipo:[-] %s\n[aqua]Tam:[-] %s\n[aqua]Mod:[-] %s\n[aqua]URL:[-] [gray]%s[-]",
				e.Name, filehunter.MediaType(e.FileType), e.Size, e.Modified, e.URL,
			))
		}
	})

	a.browserList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		idx := a.browserList.GetCurrentItem()
		if idx < 0 || idx >= len(a.fhEntries) {
			return event
		}
		entry := a.fhEntries[idx]
		switch event.Rune() {
		case 'f', 'F':
			a.addFavorite(entry)
			return nil
		case 'l', 'L':
			a.loadIntoEmulator(entry)
			return nil
		case 'd', 'D':
			a.downloadFile(entry)
			return nil
		}
		if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			a.browserBack()
			return nil
		}
		return event
	})

	a.browserList.SetSelectedFunc(func(idx int, main, sec string, r rune) {
		if idx >= 0 && idx < len(a.fhEntries) {
			entry := a.fhEntries[idx]
			if entry.IsDir {
				a.browserStack = append(a.browserStack, entry.URL)
				a.browserNavigate(entry.URL)
			}
		}
	})

	topBar := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.browserPath, 3, 0, false).
		AddItem(a.searchInput, 3, 0, false)

	left := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topBar, 6, 0, false).
		AddItem(a.browserList, 0, 1, true)

	a.browserPanel = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(left, 0, 2, true).
		AddItem(a.browserInfo, 36, 0, false)
}

// ── Favorites Panel ───────────────────────────────────────────────────────

func (a *App) buildFavoritesPanel() {
	a.favList = tview.NewList().ShowSecondaryText(true).SetHighlightFullLine(true)
	a.favList.SetBorder(true).SetTitle(" Favoritos (Enter=carregar, Del=remover) ").SetBorderColor(colorBorder)

	help := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[gray]Enter[-]: Carregar no emulador  [gray]Delete[-]: Remover favorito  [gray]F[-]: Copiar URL")
	help.SetBorder(true).SetBorderColor(colorBorder)

	a.favList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		idx := a.favList.GetCurrentItem()
		switch event.Key() {
		case tcell.KeyDelete:
			if idx >= 0 && idx < len(a.favEntries) {
				_ = a.database.DeleteFavorite(a.favEntries[idx].ID)
				a.refreshFavorites()
			}
			return nil
		}
		return event
	})

	a.favList.SetSelectedFunc(func(idx int, main, sec string, r rune) {
		if idx >= 0 && idx < len(a.favEntries) {
			fav := a.favEntries[idx]
			entry := filehunter.Entry{
				Name:     fav.FileName,
				URL:      fav.URL,
				FileType: fav.FileType,
			}
			a.loadIntoEmulator(entry)
		}
	})

	a.favPanel = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.favList, 0, 1, true).
		AddItem(help, 3, 0, false)
}

// ── History Panel ─────────────────────────────────────────────────────────

func (a *App) buildHistoryPanel() {
	a.histList = tview.NewList().ShowSecondaryText(true).SetHighlightFullLine(true)
	a.histList.SetBorder(true).SetTitle(" Histórico de Comandos (Enter=reenviar) ").SetBorderColor(colorBorder)

	a.histList.SetSelectedFunc(func(idx int, main, sec string, r rune) {
		cmd := strings.TrimSpace(main)
		a.switchTab(tabControl)
		a.sendCommand(cmd)
	})

	help := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[gray]Enter[-]: Reenviar comando  [gray]C[-]: Limpar histórico")
	help.SetBorder(true).SetBorderColor(colorBorder)

	a.histPanel = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.histList, 0, 1, true).
		AddItem(help, 3, 0, false)
}

// ── Log Panel ─────────────────────────────────────────────────────────────

func (a *App) buildLogPanel() {
	a.logView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true).
		SetChangedFunc(func() { a.tv.Draw() })
	a.logView.SetBorder(true).SetTitle(" Logs do openMSX ").SetBorderColor(colorBorder)

	a.logPanel = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.logView, 0, 1, true)
}

func (a *App) buildStatusBar() {
	a.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetText("[gray] F1:Controle F2:Conectar F3:Browser F4:Favoritos F5:Histórico F6:Logs F7:Tamanho F10:Sair [-]")
}

// ── Global Key Handler ────────────────────────────────────────────────────

func (a *App) globalKeys(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyF1:
		a.switchTab(tabControl)
		return nil
	case tcell.KeyF2:
		a.connectDialog()
		return nil
	case tcell.KeyF3:
		a.switchTab(tabBrowser)
		if len(a.fhEntries) == 0 {
			a.browserNavigate("") // browserNavigate spawns its own goroutine
		}
		return nil
	case tcell.KeyF4:
		a.switchTab(tabFavorites)
		a.refreshFavorites()
		return nil
	case tcell.KeyF5:
		a.switchTab(tabHistory)
		a.refreshHistory()
		return nil
	case tcell.KeyF6:
		a.switchTab(tabLogs)
		return nil
	case tcell.KeyF7:
		cols, rows := termSize()
		var suggestion string
		switch {
		case rows < 24:
			suggestion = "[red]⚠ Muito pequeno! Aumente para pelo menos 28 linhas.[-]"
		case rows < 28:
			suggestion = "[yellow]⚠ Pequeno. Recomendado: 28+ linhas para melhor experiência.[-]"
		case rows < 40:
			suggestion = "[green]✓ Tamanho adequado.[-]"
		default:
			suggestion = "[green]✓ Ótimo tamanho![-]"
		}
		a.setStatus(fmt.Sprintf("[gray]Terminal: %dx%d — %s", cols, rows, suggestion))
		return nil
	case tcell.KeyF10:
		a.tv.Stop()
		return nil
	}
	return event
}

// ── Connection Dialog ─────────────────────────────────────────────────────

func (a *App) connectDialog() {
	form := tview.NewForm()
	addrField := tview.NewInputField().
		SetLabel("Endereço TCP (host:porta): ").
		SetText("127.0.0.1:9938").
		SetFieldWidth(24)

	form.AddFormItem(addrField)
	form.AddButton("Auto-detectar", func() {
		a.setStatus("[yellow]Procurando openMSX nas portas 9938-9958...")
		go func() {
			addr, err := openmsx.ScanPorts()
			a.tv.QueueUpdateDraw(func() {
				if err != nil {
					a.setStatus("[red]Nenhum openMSX encontrado. Inicie o emulador primeiro.")
				} else {
					addrField.SetText(addr)
					a.setStatus("[green]openMSX encontrado em " + addr)
				}
			})
		}()
	})
	form.AddButton("Conectar", func() {
		addr := strings.TrimSpace(addrField.GetText())
		a.tv.QueueUpdateDraw(func() {
			a.pages.RemovePage("dialog")
		})
		a.doConnect(addr)
	})
	form.AddButton("Cancelar", func() {
		a.pages.RemovePage("dialog")
	})

	form.SetBorder(true).SetTitle(" Conectar ao openMSX ").SetBorderColor(colorBorder)
	form.SetButtonsAlign(tview.AlignCenter)

	modal := centeredModal(form, 60, 12)
	a.pages.AddPage("dialog", modal, true, true)
	a.tv.SetFocus(form)
}

func (a *App) doConnect(addr string) {
	a.setStatus(fmt.Sprintf("[yellow]Conectando a %s...", addr))
	go func() {
		client, err := openmsx.Connect(addr)
		a.tv.QueueUpdateDraw(func() {
			if err != nil {
				a.setStatus(fmt.Sprintf("[red]Erro ao conectar: %v", err))
				return
			}
			// If there was a previous client, disconnect it first
			if a.client != nil {
				a.client.Disconnect()
			}
			a.client = client
			a.connected = true
			a.currentAddr = addr
			a.connInfo.SetText(a.connInfoText())
			a.setStatus(fmt.Sprintf("[green]Conectado a %s", addr))
			a.appendOutput(fmt.Sprintf("[green]✓ Conectado ao openMSX em %s[-]", addr))
			// Start watchers only after a.client is safely set in UI goroutine
			go a.watchUpdates(client)
			go a.watchLogs(client)
		})
	}()
}

func (a *App) watchUpdates(c *openmsx.Client) {
	for {
		select {
		case u, ok := <-c.Updates:
			if !ok {
				return
			}
			a.tv.QueueUpdateDraw(func() {
				a.leds[u.Name] = u.Value
				a.refreshLEDs()
			})
		case <-c.Done():
			a.tv.QueueUpdateDraw(func() {
				a.connected = false
				a.connInfo.SetText(a.connInfoText())
				a.setStatus("[red]● openMSX desconectou.")
			})
			return
		}
	}
}

func (a *App) watchLogs(c *openmsx.Client) {
	for {
		select {
		case msg, ok := <-c.Logs:
			if !ok {
				return
			}
			a.tv.QueueUpdateDraw(func() {
				a.appendLog(msg)
			})
		case <-c.Done():
			return
		}
	}
}

func (a *App) refreshLEDs() {
	if len(a.leds) == 0 {
		return
	}
	var sb strings.Builder
	for name, val := range a.leds {
		color := "red"
		if val == "on" {
			color = "green"
		}
		sb.WriteString(fmt.Sprintf("[%s]● %s[-]  ", color, strings.ToUpper(name)))
	}
	a.ledStatus.SetText(sb.String())
}

// ── Command Sending ───────────────────────────────────────────────────────

func (a *App) sendCommand(cmd string) {
	if !a.connected || a.client == nil {
		a.appendOutput("[red]Não conectado. Pressione F2 para conectar.[-]")
		return
	}
	a.appendOutput(fmt.Sprintf("[aqua]> %s[-]", cmd))
	go func() {
		r := a.client.Send(cmd)
		a.tv.QueueUpdateDraw(func() {
			color := "green"
			prefix := "✓"
			if !r.OK {
				color = "red"
				prefix = "✗"
			}
			text := r.Text
			if text == "" {
				text = "(ok)"
			}
			a.appendOutput(fmt.Sprintf("[%s]%s %s[-]", color, prefix, text))
			_ = a.database.AddCommandHistory(cmd, r.Text)
		})
	}()
}

func (a *App) appendOutput(text string) {
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[gray]%s[-] %s\n", ts, text)
	fmt.Fprint(a.cmdOutput, line)
	// Cap output buffer at ~500 lines to avoid unbounded memory growth
	a.capTextView(a.cmdOutput, 500)
	a.cmdOutput.ScrollToEnd()
}

func (a *App) appendLog(msg string) {
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[gray]%s[-] %s\n", ts, msg)
	fmt.Fprint(a.logView, line)
	a.capTextView(a.logView, 500)
	a.logView.ScrollToEnd()
}

// capTextView trims the text view to at most maxLines lines.
func (a *App) capTextView(tv *tview.TextView, maxLines int) {
	text := tv.GetText(false)
	lines := strings.Split(text, "\n")
	if len(lines) > maxLines {
		tv.SetText(strings.Join(lines[len(lines)-maxLines:], "\n"))
	}
}

// ── File Browser Actions ──────────────────────────────────────────────────

func (a *App) browserNavigate(path string) {
	a.tv.QueueUpdateDraw(func() {
		a.browserList.Clear()
		a.browserList.AddItem("[gray]Carregando...", "", 0, nil)
	})

	go func() {
		entries, err := a.fhClient.List(path)
		a.tv.QueueUpdateDraw(func() {
			a.browserList.Clear()
			a.fhEntries = entries

			if err != nil {
				a.setStatus(fmt.Sprintf("[red]Erro ao carregar: %v", err))
				a.browserList.AddItem("[red]Erro ao carregar o diretório", err.Error(), 0, nil)
				return
			}

			displayPath := path
			if displayPath == "" {
				displayPath = "/"
			}
			a.browserPath.SetText(fmt.Sprintf("[aqua]%s[-]", displayPath))

			for _, e := range entries {
				icon := "📄"
				if e.IsDir {
					icon = "📁"
				}
				main := fmt.Sprintf("%s %s", icon, e.Name)
				sec := fmt.Sprintf("   %s  %s  %s", filehunter.MediaType(e.FileType), e.Size, e.Modified)
				a.browserList.AddItem(main, sec, 0, nil)
			}

			a.setStatus(fmt.Sprintf("[green]%d itens carregados de %s", len(entries), displayPath))
		})
	}()
}

func (a *App) browserBack() {
	if len(a.browserStack) > 1 {
		a.browserStack = a.browserStack[:len(a.browserStack)-1]
		parent := a.browserStack[len(a.browserStack)-1]
		a.browserNavigate(parent)
	}
}

func (a *App) browserSearch() {
	query := strings.TrimSpace(a.searchInput.GetText())
	if query == "" {
		return
	}
	current := ""
	if len(a.browserStack) > 0 {
		current = a.browserStack[len(a.browserStack)-1]
	}

	a.tv.QueueUpdateDraw(func() {
		a.browserList.Clear()
		a.browserList.AddItem("[gray]Buscando...", "", 0, nil)
	})

	go func() {
		entries, err := a.fhClient.Search(current, query)
		a.tv.QueueUpdateDraw(func() {
			a.browserList.Clear()
			a.fhEntries = entries
			if err != nil {
				a.setStatus("[red]Erro na busca: " + err.Error())
				return
			}
			for _, e := range entries {
				icon := "📄"
				if e.IsDir {
					icon = "📁"
				}
				a.browserList.AddItem(fmt.Sprintf("%s %s", icon, e.Name),
					fmt.Sprintf("   %s  %s", filehunter.MediaType(e.FileType), e.Size), 0, nil)
			}
			a.setStatus(fmt.Sprintf("[green]%d resultados para '%s'", len(entries), query))
		})
	}()
}

func (a *App) addFavorite(entry filehunter.Entry) {
	if entry.IsDir {
		a.setStatus("[yellow]Não é possível favoritar diretórios.")
		return
	}
	err := a.database.AddFavorite(entry.Name, entry.Name, entry.FileType, entry.URL, "")
	if err != nil {
		a.setStatus("[red]Erro ao salvar favorito: " + err.Error())
	} else {
		a.setStatus(fmt.Sprintf("[green]★ %s adicionado aos favoritos!", entry.Name))
	}
}

func (a *App) loadIntoEmulator(entry filehunter.Entry) {
	if entry.IsDir {
		return
	}
	if !a.connected || a.client == nil {
		a.setStatus("[red]Não conectado ao openMSX!")
		return
	}

	// Download to temp dir first
	tmpDir := os.TempDir()
	destPath := filepath.Join(tmpDir, entry.Name)

	a.setStatus(fmt.Sprintf("[yellow]Baixando %s...", entry.Name))

	go func() {
		data, err := a.fhClient.Download(entry.URL)
		if err != nil {
			a.tv.QueueUpdateDraw(func() {
				a.setStatus(fmt.Sprintf("[red]Erro ao baixar: %v", err))
			})
			return
		}

		if err := os.WriteFile(destPath, data, 0644); err != nil {
			a.tv.QueueUpdateDraw(func() {
				a.setStatus(fmt.Sprintf("[red]Erro ao salvar: %v", err))
			})
			return
		}

		var r openmsx.Reply
		ext := strings.ToLower(entry.FileType)
		switch ext {
		case "rom", "mx1", "mx2":
			r = a.client.LoadROM(destPath)
		case "dsk":
			r = a.client.LoadDisk(destPath)
		case "cas":
			r = a.client.LoadCassette(destPath)
		default:
			r = a.client.LoadROM(destPath)
		}

		_ = a.database.UpsertRecentFile(entry.Name, destPath, entry.FileType)

		a.tv.QueueUpdateDraw(func() {
			if r.OK {
				a.setStatus(fmt.Sprintf("[green]✓ %s carregado no openMSX!", entry.Name))
				a.appendOutput(fmt.Sprintf("[green]✓ Carregado: %s[-]", entry.Name))
			} else {
				a.setStatus(fmt.Sprintf("[red]Erro ao carregar: %s", r.Text))
			}
		})
	}()
}

func (a *App) downloadFile(entry filehunter.Entry) {
	if entry.IsDir {
		return
	}
	homeDir, _ := os.UserHomeDir()
	dlDir := filepath.Join(homeDir, "Downloads")
	_ = os.MkdirAll(dlDir, 0755) // create ~/Downloads if missing
	destPath := filepath.Join(dlDir, entry.Name)
	a.setStatus(fmt.Sprintf("[yellow]Baixando %s para ~/Downloads...", entry.Name))

	go func() {
		data, err := a.fhClient.Download(entry.URL)
		if err != nil {
			a.tv.QueueUpdateDraw(func() {
				a.setStatus(fmt.Sprintf("[red]Erro ao baixar: %v", err))
			})
			return
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			a.tv.QueueUpdateDraw(func() {
				a.setStatus(fmt.Sprintf("[red]Erro ao salvar: %v", err))
			})
			return
		}
		a.tv.QueueUpdateDraw(func() {
			a.setStatus(fmt.Sprintf("[green]✓ Salvo em %s", destPath))
		})
	}()
}

// ── History & Favorites refresh ───────────────────────────────────────────

func (a *App) refreshHistory() {
	entries, err := a.database.GetCommandHistory(200)
	if err != nil {
		return
	}
	a.histList.Clear()
	for _, e := range entries {
		ts := e.CreatedAt.Format("15:04:05")
		result := e.Response
		if len(result) > 60 {
			result = result[:60] + "..."
		}
		a.histList.AddItem(
			" "+e.Command,
			fmt.Sprintf("  [gray]%s  %s[-]", ts, result),
			0, nil,
		)
	}
}

func (a *App) refreshFavorites() {
	favs, err := a.database.GetFavorites()
	if err != nil {
		return
	}
	a.favEntries = favs
	a.favList.Clear()
	for _, f := range favs {
		icon := "📄"
		switch f.FileType {
		case "rom":
			icon = "🎮"
		case "dsk":
			icon = "💾"
		case "cas":
			icon = "📼"
		}
		a.favList.AddItem(
			fmt.Sprintf("%s %s", icon, f.Name),
			fmt.Sprintf("   [gray]%s  %s[-]", filehunter.MediaType(f.FileType), f.CreatedAt.Format("02/01/2006")),
			0, nil,
		)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────

func (a *App) setStatus(msg string) {
	keys := "[gray] F1:Controle F2:Conectar F3:Browser F4:Favoritos F5:Histórico F6:Logs F10:Sair [-]"
	a.statusBar.SetText(fmt.Sprintf("%s  %s", msg, keys))
}

func centeredModal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 0, true).
			AddItem(nil, 0, 1, false), width, 0, true).
		AddItem(nil, 0, 1, false)
}
