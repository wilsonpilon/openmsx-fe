# openMSX Frontend

Um frontend TUI (Text User Interface) clássico para o emulador openMSX, com visual inspirado nas interfaces Borland dos anos 90.

## Características

- Interface de texto colorida estilo Borland (azul marinho e cyan)
- Menus navegáveis com teclado
- Barra de status com atalhos
- Configurações persistentes em SQLite
- Execução do openMSX diretamente pela interface

## Requisitos

- Go 1.26 ou superior
- openMSX instalado e acessível no PATH (ou caminho configurado)
- GCC (necessário para compilar o driver SQLite no Windows)

## Instalação

1. Clone ou baixe este repositório
2. Instale as dependências:
```bash
go mod tidy
```

3. Execute o programa:
```bash
go run main.go
```

Ou compile:
```bash
go build -o openmsx-fe.exe
./openmsx-fe.exe
```

## Uso

### Navegação

- **Setas ↑/↓**: Navegar pelo menu
- **Enter**: Selecionar opção
- **E**: Executar openMSX
- **C**: Abrir configurações
- **Q**: Sair
- **F10**: Sair

### Opções do Menu

1. **Executar openMSX**: Inicia o emulador openMSX sem parâmetros adicionais
2. **Configurações**: Permite configurar o caminho do executável do openMSX
3. **Sair**: Fecha a aplicação

### Configurações

Na tela de configurações você pode:
- Definir o caminho completo para o executável do openMSX
- Salvar as configurações (persistidas em SQLite)
- Cancelar alterações

## Estrutura do Projeto

```
openmsx-fe/
├── main.go           # Código principal da aplicação
├── go.mod            # Dependências do projeto
├── go.sum            # Checksums das dependências
├── openmsx-fe.db     # Banco de dados SQLite (criado automaticamente)
└── README.md         # Este arquivo
```

## Dependências

- [tview](https://github.com/rivo/tview) - Framework TUI
- [tcell](https://github.com/gdamore/tcell) - Biblioteca de terminal
- [go-sqlite3](https://github.com/mattn/go-sqlite3) - Driver SQLite para Go

## Banco de Dados

O programa cria automaticamente um arquivo `openmsx-fe.db` no diretório de execução para armazenar:
- Caminho do executável openMSX
- Futuras configurações adicionais

## Desenvolvimento

### Estrutura do Código

- `initDB()`: Inicializa o banco de dados SQLite
- `saveConfig()`: Salva configurações no banco
- `runOpenMSX()`: Executa o emulador openMSX
- `showConfigScreen()`: Exibe a tela de configurações
- `createMainScreen()`: Cria a interface principal
- `main()`: Ponto de entrada da aplicação

### Cores do Tema Borland

- Background: Navy Blue (`tcell.ColorNavy`)
- Foreground: Teal/Cyan (`tcell.ColorTeal`)
- Text: White (`tcell.ColorWhite`)
- Labels: Yellow (`tcell.ColorYellow`)

## Roadmap

Funcionalidades planejadas para futuras versões:
- [ ] Gerenciamento de ROMs
- [ ] Histórico de jogos recentes
- [ ] Configurações de emulação
- [ ] Favoritos
- [ ] Capturas de tela
- [ ] Save states

## Licença

Este projeto é software livre. Use como desejar.

## Sobre o openMSX

openMSX é um emulador de código aberto do MSX, que visa alcançar perfeita emulação.
Visite: https://openmsx.org/

## Autor

Criado como um projeto educacional para demonstrar interfaces TUI em Go.
