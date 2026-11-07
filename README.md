# Kaelix · Industrial Intelligence

Aplicação desktop de monitoramento industrial com Wails 2, Go, React e TypeScript.
A interface segue as referências visuais em `.temp`: tema escuro, workflow 2D,
telemetria lateral e estados de operação em verde e âmbar. A identidade combina
laranja, grafite e um símbolo geométrico próprio; consulte `docs/identidade-visual.md`. A janela usa uma área de trabalho
fixa, navegação lateral recolhível, painéis com rolagem independente, barra de estado
e atalhos Ctrl/⌘ + 1 a 9 e 0. No macOS, o cabeçalho integra a barra nativa
e o menu Área de trabalho permite trocar de seção. O canvas usa SVG e cards interativos;
as imagens de referência não são distribuídas na aplicação.

## Executar

Requisitos: Go 1.26.6+, Node 20.19+ ou 22.12+, npm e Wails CLI 2.13.
Os requisitos nativos variam por sistema operacional.

```sh
cd apps/desktop
wails dev
```

Prévia apenas da interface, sem o runtime desktop:

```sh
cd apps/desktop/frontend
npm ci
npm run dev
```

Build desktop:

```sh
cd apps/desktop
wails build
```

## Estrutura

- `apps/desktop/main.go`: janela nativa e assets embutidos.
- `apps/desktop/app.go`: ciclo de vida Go e metadados expostos ao frontend.
- `apps/desktop/frontend/src/App.tsx`: navegação e telas iniciais.
- `apps/desktop/frontend/src/components`: planta SVG e gráfico com suporte a lacunas.
- `apps/desktop/app_api.go`: sessão, consultas e cadastro na API.
- `apps/desktop/internal/apiclient`: cliente HTTP com timeout e cancelamento.
- `apps/desktop/frontend/src/lib/useWorkspace.ts`: atualização das telas e histórico.
- `apps/desktop/frontend/src/lib/types.ts`: apresentação do contrato e exportação CSV.
- `apps/desktop/app_field.go` e `export.go`: gateways, transmissores, coletas, pareamento e exportação.
- `docs/comando-e-coleta.md`: especificação do enlace com o firmware.
- `docs/arquitetura-desktop-internet.md`: arquitetura e histórico das etapas.

## Montagem em campo

O computador vai a campo ligado à tomada, na mesma rede Wi-Fi do gateway. O transmissor ESP32-C3 envia
telemetria e sinal bruto por LoRa ao gateway ESP32-S3, que fala com a API neste Mac pela rede local, sem
cabo, túnel ou nuvem. A especificação é [Comando e coleta sem cabo](docs/comando-e-coleta.md).

## Escopo atual

Desktop conectado à API Go: sensores/motores, workflow dinâmico, dashboard,
saúde dos gateways, últimas recepções, histórico e exportação CSV com horários UTC.
As telas consultam a API a cada dois segundos, sem requisições de polling sobrepostas;
a janela oculta pausa consultas. Leituras do gateway são recebidas e
persistidas no PostgreSQL deste Mac.

Tudo se configura pelo desktop: **Gateways** mostra estado, rede e fila em flash, com o assistente
"Adicionar gateway", redes Wi-Fi e envio de firmware. **Transmissores** tem o formulário da
configuração, o editor da rotina, o estado pendente, enviado ou aplicado, e os comandos. **Coletas**
inicia e encerra com nome e condição, resume as perdas e exporta tudo para uma pasta. **Sinal** mostra
cada canal da janela escolhida.

Não há fallback de dados fictícios. API indisponível mantém a última consulta
identificada como desatualizada; sessão ausente/expirada apresenta a conexão.
`UNKNOWN` preserva a classificação original; temperatura ausente fica vazia.
Sensor sem vínculo aparece pelo ID LoRa e pode ser associado a um motor em
**Motores → Associar a um motor**. O cadastro/edição persiste na API.

Histórico por dispositivo com períodos de 15 min, 1 h e 24 h, até mil leituras
por página e botão para carregar anteriores. O CSV exporta as leituras carregadas,
com RMS em g, temperatura em °C, diagnóstico, metadados de recepção e atraso da fila.
Eventos mostra as últimas 50 recepções reais; alertas derivados não são fabricados.

Em **Configurações**, informar URL/e-mail/senha da conta da API; o endereço padrão é
`http://127.0.0.1:8080`. A senha nunca é persistida; sessão e endereço ficam no SQLite privado do
usuário até logout ou expiração de 30 dias. Se o endereço salvo não responder e houver uma API neste
computador, o aplicativo oferece trocar para ela. Tokens de gateway são exclusivos do firmware, não
do login desktop.

## Perfil e banco local

O modal de Perfil (avatar na barra superior ou Ctrl/⌘ + 0) salva nome, e-mail, empresa e cargo em SQLite,
com migração automática no startup. O arquivo fica em `Kaelix/kaelix.db`
dentro de `os.UserConfigDir()`; no macOS, `~/Library/Application Support/Kaelix/`.
Requisito Go atualizado para 1.26 pelo driver SQLite.

O perfil é local e não é uma conta autenticada. Nenhuma senha é armazenada.
API, PostgreSQL central e persistência de medições estão disponíveis em `apps/server`.
O desktop consulta essa API por seu backend Go e restaura sessões válidas entre reinícios.
No navegador sem Wails, o perfil informa a indisponibilidade do backend;
não substitui a persistência por dados simulados.

A foto do perfil pode ser escolhida pelo seletor nativo (PNG/JPEG até 10 MB),
recortada ao centro em 256×256 e armazenada no SQLite. Escape ou clique no
fundo fecha o modal.

## API Go e banco central

Servidor independente em `apps/server`, com PostgreSQL, migrações, cadastro de
motores/dispositivos/gateways, ingestão idempotente e histórico por período/cursor.
Login administrativo e credenciais individuais de gateway protegem os endpoints.

Consulte [execução e contrato HTTP](apps/server/README.md). A API escuta em HTTP só neste computador,
para o desktop, e em HTTPS na rede local, só com as rotas de gateway. No Mac de campo, instale o serviço
com `scripts/install-launchagent.sh`. O ambiente em contêiner está em `deploy/compose.yaml`; configure
`deploy/.env` a partir de `.env.example`.
Para verificar com um PostgreSQL temporário, execute:

```sh
./scripts/test-server-postgres.sh
```
