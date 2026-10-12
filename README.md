# Kaelix · Industrial Intelligence

Boilerplate desktop de monitoramento industrial com Wails 2, Go, React e TypeScript.
A interface segue as referências visuais em `.temp`: tema escuro, workflow 2D,
telemetria lateral e estados de operação em verde e âmbar. A identidade combina
laranja, grafite e um símbolo geométrico próprio; consulte `docs/identidade-visual.md`. A janela usa uma área de trabalho
fixa, navegação lateral recolhível, painéis com rolagem independente, barra de estado
e atalhos Ctrl/⌘ + 1 a 6. No macOS, o cabeçalho integra a barra nativa
e o menu Área de trabalho permite trocar de seção. O canvas usa SVG e cards interativos;
as imagens de referência não são distribuídas na aplicação.

## Executar

Requisitos: Go 1.23+, Node 20.19+ ou 22.12+, npm e Wails CLI 2.13.
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
- `apps/desktop/frontend/src/lib/demo.ts`: dados e exportação demonstrativos.
- `docs/arquitetura-desktop-internet.md`: arquitetura e etapas de integração.

## Escopo atual

Visão geral, seleção de motor na planta, busca de equipamentos, histórico demonstrativo,
eventos, saúde de gateway e exportação CSV das últimas leituras exibidas. Configurações
salva apenas o endereço da futura API no armazenamento local; não salva credenciais.

Todos os dados são simulados e fixos. Não há API, PostgreSQL, autenticação, firmware,
polling ou classificação real implementados. O filtro de histórico alterna dois recortes
do conjunto demonstrativo. Horários e estados da tela são exemplos, não relógios ativos.
O CSV contém apenas as últimas leituras demonstrativas, sem timestamp de aquisição.

A próxima etapa é implementar contrato HTTP, API e banco conforme o documento de arquitetura.
