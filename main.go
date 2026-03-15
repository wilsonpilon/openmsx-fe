package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	_ "github.com/mattn/go-sqlite3"
)

const (
	ColorBorland   = tcell.ColorTeal
	ColorBorlandBg = tcell.ColorNavy
)

type Config struct {
	OpenMSXPath string
}

type OpenMSXBridge struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	mu     sync.Mutex
	running bool
}

var (
	db     *sql.DB
	app    *tview.Application
	pages  *tview.Pages
	config Config
	bridge *OpenMSXBridge
)

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./openmsx-fe.db")
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT
		)
	`)
	if err != nil {
		return err
	}

	// Carregar configuração
	row := db.QueryRow("SELECT value FROM config WHERE key = 'openmsx_path'")
	var path string
	if err := row.Scan(&path); err != nil {
		if err == sql.ErrNoRows {
			config.OpenMSXPath = "openmsx"
			saveConfig()
		}
	} else {
		config.OpenMSXPath = path
	}

	return nil
}

func saveConfig() error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO config (key, value)
		VALUES ('openmsx_path', ?)
	`, config.OpenMSXPath)
	return err
}

func runOpenMSX() {
	app.Suspend(func() {
		cmd := exec.Command(config.OpenMSXPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		fmt.Println("Executando openMSX...")
		if err := cmd.Run(); err != nil {
			fmt.Printf("Erro ao executar openMSX: %v\n", err)
			fmt.Println("\nPressione Enter para continuar...")
			fmt.Scanln()
		}
	})
}

func showConfigScreen() {
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Configurações ")
	form.SetTitleAlign(tview.AlignLeft)
	form.SetBackgroundColor(ColorBorlandBg)
	form.SetFieldBackgroundColor(ColorBorlandBg)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetLabelColor(tcell.ColorYellow)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetButtonBackgroundColor(ColorBorland)
	form.SetButtonTextColor(tcell.ColorWhite)

	form.AddInputField("Caminho do openMSX:", config.OpenMSXPath, 50, nil, func(text string) {
		config.OpenMSXPath = text
	})

	form.AddButton("Salvar", func() {
		if err := saveConfig(); err != nil {
			showMessage("Erro ao salvar configuração: " + err.Error())
		} else {
			pages.SwitchToPage("main")
		}
	})

	form.AddButton("Cancelar", func() {
		// Recarregar config do banco
		row := db.QueryRow("SELECT value FROM config WHERE key = 'openmsx_path'")
		var path string
		if err := row.Scan(&path); err == nil {
			config.OpenMSXPath = path
		}
		pages.SwitchToPage("main")
	})

	pages.AddPage("config", form, true, false)
	pages.SwitchToPage("config")
}

func showMessage(message string) {
	modal := tview.NewModal()
	modal.SetText(message)
	modal.AddButtons([]string{"OK"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		pages.RemovePage("message")
	})
	modal.SetBackgroundColor(ColorBorlandBg)
	modal.SetButtonBackgroundColor(ColorBorland)
	modal.SetButtonTextColor(tcell.ColorWhite)

	pages.AddPage("message", modal, true, true)
}

func (b *OpenMSXBridge) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return fmt.Errorf("openMSX já está rodando")
	}

	// Cria comando com -control stdio para comunicação via XML
	b.cmd = exec.Command(config.OpenMSXPath, "-control", "stdio")

	var err error
	b.stdin, err = b.cmd.StdinPipe()
	if err != nil {
		return err
	}

	b.stdout, err = b.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	b.stderr, err = b.cmd.StderrPipe()
	if err != nil {
		return err
	}

	// Inicia o processo em background
	if err := b.cmd.Start(); err != nil {
		return err
	}

	b.running = true

	// Goroutine para monitorar se o processo terminou
	go func() {
		b.cmd.Wait()
		b.mu.Lock()
		b.running = false
		b.mu.Unlock()
	}()

	return nil
}

func (b *OpenMSXBridge) SendCommand(command string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return fmt.Errorf("openMSX não está rodando")
	}

	xmlCommand := fmt.Sprintf("<command>%s</command>\n", command)
	_, err := b.stdin.Write([]byte(xmlCommand))
	return err
}

func (b *OpenMSXBridge) ReadOutput() (string, error) {
	if !b.running {
		return "", fmt.Errorf("openMSX não está rodando")
	}

	reader := bufio.NewReader(b.stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	// Limpa tags XML
	line = strings.ReplaceAll(line, "<reply>", "")
	line = strings.ReplaceAll(line, "</reply>", "")
	line = strings.ReplaceAll(line, "<ok>", "")
	line = strings.ReplaceAll(line, "</ok>", "")
	line = strings.TrimSpace(line)

	return line, nil
}

func (b *OpenMSXBridge) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return nil
	}

	b.running = false

	if b.stdin != nil {
		b.stdin.Close()
	}

	if b.cmd != nil && b.cmd.Process != nil {
		b.cmd.Process.Kill()
		b.cmd.Wait()
	}

	return nil
}

func (b *OpenMSXBridge) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

func showCommandTestDialog() {
	// TextView para log de saída
	outputView := tview.NewTextView()
	outputView.SetBorder(true)
	outputView.SetTitle(" Saída ")
	outputView.SetBackgroundColor(ColorBorlandBg)
	outputView.SetTextColor(tcell.ColorWhite)
	outputView.SetDynamicColors(true)
	outputView.SetScrollable(true)

	// InputField para comandos
	commandInput := tview.NewInputField()
	commandInput.SetLabel("Comando: ")
	commandInput.SetFieldWidth(60)
	commandInput.SetBackgroundColor(ColorBorlandBg)
	commandInput.SetFieldBackgroundColor(tcell.ColorBlack)
	commandInput.SetFieldTextColor(tcell.ColorWhite)
	commandInput.SetLabelColor(tcell.ColorYellow)

	// Botões
	buttonBar := tview.NewFlex()
	buttonBar.SetDirection(tview.FlexColumn)

	startBtn := tview.NewButton("Iniciar")
	startBtn.SetBackgroundColor(ColorBorland)
	startBtn.SetLabelColor(tcell.ColorWhite)
	startBtn.SetSelectedFunc(func() {
		if bridge == nil {
			bridge = &OpenMSXBridge{}
		}

		if bridge.IsRunning() {
			outputView.SetText(outputView.GetText(false) + "[yellow]openMSX já está rodando[white]\n")
			return
		}

		if err := bridge.Start(); err != nil {
			outputView.SetText(outputView.GetText(false) + fmt.Sprintf("[red]Erro: %v[white]\n", err))
		} else {
			outputView.SetText(outputView.GetText(false) + "[green]openMSX iniciado com sucesso![white]\n")

			// Thread para ler saída
			go func() {
				for bridge.IsRunning() {
					output, err := bridge.ReadOutput()
					if err != nil {
						break
					}
					if output != "" {
						app.QueueUpdateDraw(func() {
							outputView.SetText(outputView.GetText(false) + fmt.Sprintf("[cyan]> %s[white]\n", output))
						})
					}
				}
			}()
		}
	})

	sendBtn := tview.NewButton("Enviar")
	sendBtn.SetBackgroundColor(ColorBorland)
	sendBtn.SetLabelColor(tcell.ColorWhite)
	sendBtn.SetSelectedFunc(func() {
		if bridge == nil || !bridge.IsRunning() {
			outputView.SetText(outputView.GetText(false) + "[red]openMSX não está rodando![white]\n")
			return
		}

		cmd := commandInput.GetText()
		if cmd == "" {
			return
		}

		if err := bridge.SendCommand(cmd); err != nil {
			outputView.SetText(outputView.GetText(false) + fmt.Sprintf("[red]Erro: %v[white]\n", err))
		} else {
			outputView.SetText(outputView.GetText(false) + fmt.Sprintf("[yellow]>> %s[white]\n", cmd))
			commandInput.SetText("")
		}
	})

	stopBtn := tview.NewButton("Parar")
	stopBtn.SetBackgroundColor(ColorBorland)
	stopBtn.SetLabelColor(tcell.ColorWhite)
	stopBtn.SetSelectedFunc(func() {
		if bridge != nil {
			bridge.Stop()
			outputView.SetText(outputView.GetText(false) + "[red]openMSX parado[white]\n")
		}
	})

	closeBtn := tview.NewButton("Fechar")
	closeBtn.SetBackgroundColor(ColorBorland)
	closeBtn.SetLabelColor(tcell.ColorWhite)
	closeBtn.SetSelectedFunc(func() {
		if bridge != nil && bridge.IsRunning() {
			bridge.Stop()
		}
		pages.SwitchToPage("main")
	})

	buttonBar.AddItem(startBtn, 0, 1, false)
	buttonBar.AddItem(tview.NewBox().SetBackgroundColor(ColorBorlandBg), 1, 0, false)
	buttonBar.AddItem(sendBtn, 0, 1, false)
	buttonBar.AddItem(tview.NewBox().SetBackgroundColor(ColorBorlandBg), 1, 0, false)
	buttonBar.AddItem(stopBtn, 0, 1, false)
	buttonBar.AddItem(tview.NewBox().SetBackgroundColor(ColorBorlandBg), 1, 0, false)
	buttonBar.AddItem(closeBtn, 0, 1, false)

	// Enter no input envia comando
	commandInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			// Enviar comando
			if bridge == nil || !bridge.IsRunning() {
				outputView.SetText(outputView.GetText(false) + "[red]openMSX não está rodando![white]\n")
				return
			}

			cmd := commandInput.GetText()
			if cmd == "" {
				return
			}

			if err := bridge.SendCommand(cmd); err != nil {
				outputView.SetText(outputView.GetText(false) + fmt.Sprintf("[red]Erro: %v[white]\n", err))
			} else {
				outputView.SetText(outputView.GetText(false) + fmt.Sprintf("[yellow]>> %s[white]\n", cmd))
				commandInput.SetText("")
			}
		}
	})

	// Layout
	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)
	flex.SetBackgroundColor(ColorBorlandBg)
	flex.AddItem(outputView, 0, 1, false)
	flex.AddItem(commandInput, 1, 0, true)
	flex.AddItem(buttonBar, 3, 0, false)
	flex.SetBorder(true)
	flex.SetTitle(" Teste de Comandos openMSX ")

	pages.AddPage("commandtest", flex, true, false)
	pages.SwitchToPage("commandtest")
}

func createMainScreen() tview.Primitive {
	// Menu principal
	menu := tview.NewList()
	menu.SetBorder(true)
	menu.SetTitle(" openMSX Frontend ")
	menu.SetTitleAlign(tview.AlignCenter)
	menu.SetBackgroundColor(ColorBorlandBg)
	menu.SetMainTextColor(tcell.ColorWhite)
	menu.SetSelectedTextColor(tcell.ColorBlack)
	menu.SetSelectedBackgroundColor(ColorBorland)
	menu.SetShortcutColor(tcell.ColorYellow)

	menu.AddItem("Executar openMSX", "", 'e', func() {
		runOpenMSX()
	})

	menu.AddItem("Teste de Comandos", "", 't', func() {
		showCommandTestDialog()
	})

	menu.AddItem("Configurações", "", 'c', func() {
		showConfigScreen()
	})

	menu.AddItem("Sair", "", 'q', func() {
		app.Stop()
	})

	// Barra de status
	statusBar := tview.NewTextView()
	statusBar.SetTextAlign(tview.AlignLeft)
	statusBar.SetBackgroundColor(ColorBorland)
	statusBar.SetTextColor(tcell.ColorWhite)
	statusBar.SetText(" F10-Sair | E-Executar | T-Teste | C-Config | Q-Sair ")

	// Layout principal
	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)
	flex.AddItem(menu, 0, 1, true)
	flex.AddItem(statusBar, 1, 0, false)
	flex.SetBackgroundColor(ColorBorlandBg)

	return flex
}

func main() {
	// Inicializar banco de dados
	if err := initDB(); err != nil {
		log.Fatalf("Erro ao inicializar banco de dados: %v", err)
	}
	defer db.Close()

	// Criar aplicação
	app = tview.NewApplication()
	pages = tview.NewPages()

	// Tela principal
	mainScreen := createMainScreen()
	pages.AddPage("main", mainScreen, true, true)

	// Atalhos globais
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyF10 {
			app.Stop()
			return nil
		}
		return event
	})

	// Executar aplicação
	if err := app.SetRoot(pages, true).EnableMouse(true).Run(); err != nil {
		log.Fatalf("Erro ao executar aplicação: %v", err)
	}
}
