# Kaelix · Industrial Intelligence

Aplicação desktop de monitoramento industrial com Wails 2, Go, React e TypeScript.
A interface segue as referências visuais em `.temp`: tema escuro, workflow 2D,
telemetria lateral e estados de operação em verde e âmbar. A identidade combina
laranja, grafite e um símbolo geométrico próprio; consulte `docs/identidade-visual.md`. A janela usa uma área de trabalho
fixa, navegação lateral recolhível, painéis com rolagem independente, barra de estado
e atalhos Ctrl/⌘ + 1 a 6. No macOS, o cabeçalho integra a barra nativa
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
- `docs/arquitetura-desktop-internet.md`: arquitetura e etapas de integração.

## Escopo atual

Desktop conectado à API Go: sensores/motores, workflow dinâmico, dashboard,
saúde dos gateways, últimas recepções, histórico e exportação CSV com horários UTC.
As telas consultam a API a cada dois segundos, sem requisições de polling sobrepostas;
a janela oculta pausa consultas. Leituras do C3 já são recebidas e
persistidas no PostgreSQL da bancada.

Não há fallback de dados fictícios. API indisponível mantém a última consulta
identificada como desatualizada; sessão ausente/expirada apresenta a conexão.
`UNKNOWN` preserva a classificação original; temperatura ausente fica vazia.
Sensor sem vínculo aparece pelo ID LoRa e pode ser associado a um motor em
**Motores → Associar a um motor**. O cadastro/edição persiste na API.

Histórico por dispositivo com períodos de 15 min, 1 h e 24 h, até mil leituras
por página e botão para carregar anteriores. O CSV exporta as leituras carregadas,
com RMS em g, temperatura em °C, diagnóstico, metadados de recepção e atraso da fila.
Eventos mostra as últimas 50 recepções reais; alertas derivados não são fabricados.

Em **Configurações**, informar URL/e-mail/senha da conta da API. A senha nunca é
persistida; sessão e endereço ficam no SQLite privado do usuário até logout ou
expiração. Tokens de gateway são exclusivos do firmware, não do login desktop.
Neste Mac, a sessão da bancada foi configurada para `http://192.168.18.167:8080`.

## Perfil e banco local

O modal de Perfil (avatar na barra superior ou Ctrl/⌘ + 7) salva nome, e-mail, empresa e cargo em SQLite,
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

Consulte [execução e contrato HTTP](apps/server/README.md). O ambiente local está
em `deploy/compose.yaml`; configure `deploy/.env` a partir de `.env.example`.
Para verificar com um PostgreSQL temporário, execute:

```sh
./scripts/test-server-postgres.sh
```
