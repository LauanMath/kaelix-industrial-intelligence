# Identidade visual · Kaelix

A identidade interpreta as referências fornecidas em `.temp/identidade`: formas
geométricas, contraste alto, laranja intenso e uma base escura neutra. O símbolo é um K
próprio, formado por uma haste e duas diagonais, sem reproduzir as marcas das referências.

| Papel | Cor |
| --- | --- |
| Marca e seleção | `#FB5102` |
| Base grafite | `#1C1C1C` |
| Texto da marca | `#CDCDCB` |
| Hover e foco | `#FF7131` |
| Estado normal | Verde |
| Estado de atenção | Âmbar |
| Estado desconhecido | Cinza azulado |

O laranja identifica comandos, seleção e foco. A cor de marca não significa
falha nem classificação do motor. As cores operacionais permanecem independentes.
O gráfico acompanha a classificação recebida; o valor numérico principal é neutro.

A assinatura usa caixa alta, espaçamento controlado e o descritor
“Industrial Intelligence”. As fontes das referências são direções visuais;
a interface usa Inter, os títulos Space Grotesk e a telemetria JetBrains Mono,
como no SOLARA. As fontes são distribuídas localmente em `public/fonts`, com suas
licenças SIL Open Font License, para funcionar sem internet.

## Assets

- `apps/desktop/frontend/public/brand/kaelix-mark.svg`: símbolo vetorial.
- `apps/desktop/frontend/src/components/Brand.tsx`: assinatura e versão compacta.
- `apps/desktop/frontend/src/brand.css`: tokens e aplicação da marca.
- `apps/desktop/build/appicon.png`: ícone desktop, gerado do mesmo desenho.
- `scripts/generate-app-icon.swift`: regeneração do PNG no macOS com AppKit.

A estrutura de área de trabalho desktop permanece a base da aplicação.

## Superfícies e ícones

Rampa neutra: `#141414`, `#1C1C1C`, `#222222`, `#282828`, `#303030`,
`#3B3B3B` e `#505050`, do fundo aos controles elevados.
Ícones de interface usam `@phosphor-icons/react`, com peso regular.
