# Desktop Kaelix

Execute `wails dev` nesta pasta para desenvolvimento, ou `wails build` para gerar
o executável. Requer a API central disponível e uma conta de usuário.

Em Configurações, conecte-se com URL/e-mail/senha da API; o endereço padrão é
`http://127.0.0.1:8080`. A senha não é salva; sessões válidas, de até 30 dias, são
restauradas do SQLite privado. Se o endereço salvo não responder e houver uma API neste
computador, um aviso oferece a troca. A sessão antiga não é enviada ao outro servidor:
um novo login é pedido.

Monitoramento, dashboard, gateways, recepções e histórico usam dados reais do
servidor. Sensores ainda não associados aparecem pelo ID e podem ser vinculados
em Motores. Nenhum conjunto demonstrativo é carregado.

O perfil no avatar continua local, separado da conta administrativa da API.
No navegador com `npm run dev`, os bindings Go não estão disponíveis: a tela
informa a indisponibilidade, sem substituir o backend por dados fictícios.

Verificação: primeiro `npm run build` em `frontend/`; depois `go test -race ./...`
e `go vet ./...` nesta pasta. Não executar o build do frontend simultaneamente
aos testes Go: o embed do Wails lê o diretório `frontend/dist`.

Consulte o [README da raiz](../../README.md), o [guia da API](../server/README.md) e a
[especificação do enlace](../../docs/comando-e-coleta.md).

## Comando e coleta

**Gateways:** o assistente "Adicionar gateway" cria o gateway na API, pede que o Mac entre na rede
`Kaelix-XXXX`, lê `/info` e `/scan` em `http://192.168.4.1` pelo backend Go, com tempos curtos, envia
`/provision`, acompanha `/info` e aguarda o primeiro heartbeat. A API continua acessível nesse meio
tempo porque roda neste Mac. O token do gateway fica só na memória do backend. A chave de administração
fica no SQLite local: sem ela, um gateway já pareado pede o botão BOOT por 10 s. No macOS, o sistema
pode pedir permissão de Rede Local na primeira conexão ao gateway.

**Transmissores:** o formulário usa as mesmas regras da API, em `packages/contracts`, e mostra cada erro
junto do campo e o intervalo mínimo calculado. O estado distingue pendente, enviado pelo rádio e
aplicado; só a telemetria aplica.

**Coletas e exportação:** a exportação pede uma pasta no diálogo nativo e percorre todas as páginas da
API. Cria `coleta-<id>-<nome>/` com `leituras.csv`, `janelas.csv` e `janela-<id>.csv`, em CSV com ponto e
vírgula, separador decimal ponto e horários UTC. Cada janela traz os valores brutos e em unidades físicas.
A pasta só recebe o nome final quando termina.

**Sinal:** um gráfico por canal. Amostras de um fragmento perdido chegam como zero e não são dados: ficam
vazias na exportação e em faixa cinza no gráfico.

## Logs do gateway

Clique no card do gateway no workflow para abrir o card de logs ao lado da topologia. Na aba **Gateways**, clique no card para expandir seus logs logo abaixo. O console usa a sessão da API já configurada e acompanha somente o gateway selecionado, com níveis DEBUG/INFO/WARN/ERROR, busca, pausa, rolagem automática, limpeza local e exportação `.log`.

A consulta ocorre a cada dois segundos, enquanto a janela estiver visível e o console não estiver pausado. O gateway envia lotes de logs a cada cinco segundos, então a visualização tem atraso de rede/lote. A primeira consulta carrega até 200 linhas recentes; a tela mantém até 2.000. Timestamp `+HH:MM:SS.mmm` é tempo desde o boot do gateway; o tooltip e a exportação incluem horário de chegada à API, sessão e sequência. Lacunas na sequência de logs são sinalizadas. A pausa/limpeza não altera medições nem apaga registros no servidor.

Requer API com `/api/v1/gateways/{id}/logs` e firmware do gateway atualizado com `/api/v1/ingest/logs`. Firmware antigo só envia heartbeat e medições; nenhum log serial anterior é reconstruído ou simulado. Logs internos do transmissor e mensagens do ROM não estão no transporte atual. Se a rede cair, só as linhas ainda disponíveis na fila RAM do gateway poderão chegar após reconectar.

## Perfil e banco local

O perfil (avatar na barra superior ou Ctrl/⌘ + 0) guarda nome, e-mail, empresa, cargo e foto num SQLite em
`Kaelix/kaelix.db`, dentro de `os.UserConfigDir()`; no macOS, `~/Library/Application Support/Kaelix/`. O
mesmo arquivo guarda a sessão da API e as chaves de administração dos gateways pareados, e as migrações
rodam na abertura. O perfil é local e não é uma conta da API; nenhuma senha é armazenada.

A foto é escolhida no seletor nativo (PNG ou JPEG até 10 MB) e recortada ao centro em 256 × 256 pixels. Escape ou
um clique fora fecha o modal.
