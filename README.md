# MSXFront 🕹️

**Frontend TUI para o emulador openMSX — com controle em tempo real e integração ao FileHunter**

```
 ███╗   ███╗███████╗██╗  ██╗    ██╗      █████╗ ██╗   ██╗███╗   ██╗
 ████╗ ████║██╔════╝╚██╗██╔╝    ██║     ██╔══██╗██║   ██║████╗  ██║
 ██╔████╔██║███████╗ ╚███╔╝     ██║     ███████║██║   ██║██╔██╗ ██║
 ██║╚██╔╝██║╚════██║ ██╔██╗     ██║     ██╔══██║██║   ██║██║╚██╗██║
 ██║ ╚═╝ ██║███████║██╔╝ ██╗    ███████╗██║  ██║╚██████╔╝██║ ╚████║
 ╚═╝     ╚═╝╚══════╝╚═╝  ╚═╝    ╚══════╝╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝
```

---

## Funcionalidades

| Feature | Descrição |
|---|---|
| 🎮 **Controle Total** | Envie qualquer comando Tcl ao openMSX em tempo real |
| ⚡ **Botões Rápidos** | Power ON/OFF, Reset, Pause, Rewind, Screenshot |
| 💡 **LEDs ao Vivo** | Monitor de LEDs (power, caps, kana, turbo) via updates XML |
| 💾 **Savestates** | Salve e carregue estados da máquina |
| 🔍 **Browser FileHunter** | Navegue em download.file-hunter.com direto do TUI |
| ⬇️ **Download & Load** | Baixe ROMs/DSKs e carregue no emulador com 1 tecla |
| ⭐ **Favoritos** | Salve jogos favoritos no SQLite |
| 📜 **Histórico** | Todos os comandos enviados com timestamp e resposta |
| 📋 **Logs** | Log em tempo real do openMSX |

---

## Pré-requisitos

- **Go 1.21+** — https://go.dev/dl/
- **GCC/CGO** — necessário para go-sqlite3
  - Windows: [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) ou MSYS2
  - Linux: `sudo apt install gcc`
- **openMSX** rodando com suporte a socket TCP

---

## Compilação

```bash
# Clone ou extraia o projeto
cd msxfront

# Opção 1: script automático
chmod +x build.sh
./build.sh

# Opção 2: manual
go mod tidy
go build -o msxfront ./cmd/main.go

# Windows (CGO necessário)
set CGO_ENABLED=1
go build -o msxfront.exe ./cmd/main.go
```

---

## Configurando o openMSX para aceitar conexões TCP

No Windows, o openMSX expõe um socket TCP nas portas **9938–9958**.

**Inicie o openMSX normalmente** (sem flags especiais) — o socket TCP é criado automaticamente.

> 💡 O MSXFront inclui **auto-detecção**: pressione `F2` → `Auto-detectar` e ele
> escaneia as 21 portas automaticamente.

Se preferir iniciar via linha de comando:
```
openmsx.exe -machine msx2 -carta "C:\roms\gradius.rom"
```

---

## Uso

```bash
./msxfront        # Linux/macOS
msxfront.exe      # Windows
```

### Navegação por Teclado

| Tecla | Ação |
|---|---|
| `F1` | Painel de Controle |
| `F2` | Conectar/Desconectar openMSX |
| `F3` | Browser FileHunter |
| `F4` | Favoritos |
| `F5` | Histórico de Comandos |
| `F6` | Logs do openMSX |
| `F10` | Sair |

### No Browser FileHunter

| Tecla | Ação |
|---|---|
| `Enter` | Abrir diretório |
| `L` | Carregar arquivo no openMSX |
| `F` | Adicionar aos favoritos |
| `D` | Baixar para ~/Downloads |
| `Backspace` | Voltar ao diretório anterior |

### Comandos Tcl Comuns

```tcl
set power on          # Liga o MSX
set power off         # Desliga
reset                 # Reset do MSX
set pause on          # Pausa emulação
set pause off         # Retoma emulação
set throttle on       # Limita velocidade ao real
set throttle off      # Sem limite de velocidade
reverse enable        # Ativa rewind
reverse disable       # Desativa rewind
carta /path/game.rom  # Insere ROM no slot A
diska /path/disk.dsk  # Insere disco
savestate myjogo      # Salva estado
loadstate myjogo      # Carrega estado
screenshot            # Captura tela
machine_info name     # Nome da máquina atual
```

---

## Estrutura do Projeto

```
msxfront/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── db/
│   │   └── db.go            # SQLite: histórico, favoritos, recentes
│   ├── openmsx/
│   │   └── client.go        # Protocolo XML TCP openMSX
│   ├── filehunter/
│   │   └── client.go        # HTTP browser download.file-hunter.com
│   └── ui/
│       └── app.go           # TUI tview — layout e eventos
├── go.mod
├── build.sh
└── README.md
```

---

## Protocolo openMSX

O MSXFront comunica com o openMSX via **XML sobre TCP**:

```xml
<!-- Envio -->
<openmsx-control>
<command>set power on</command>

<!-- Resposta -->
<openmsx-output>
<reply result="ok"></reply>

<!-- Updates assíncronos -->
<update type="led" machine="machine1" name="power">on</update>
```

---

## Banco de Dados SQLite

O arquivo `msxfront.db` é criado automaticamente com as tabelas:

- `command_history` — histórico de comandos com timestamp e resposta
- `favorites` — jogos e arquivos favoritos
- `recent_files` — arquivos carregados recentemente

---

## Licença

MIT — use livremente para sua coleção de MSX! 🎮