# Desktop Kaelix

Execute `wails dev` nesta pasta para desenvolvimento, ou `wails build` para gerar
o executável. Requer a API central disponível e uma conta de usuário.

Em Configurações, conecte-se com URL/e-mail/senha da API. A senha não é salva;
sessões válidas são restauradas do SQLite privado. Neste Mac, a conta da bancada
já está configurada para `http://192.168.18.167:8080`.

Monitoramento, dashboard, gateways, recepções e histórico usam dados reais do
servidor. Sensores ainda não associados aparecem pelo ID e podem ser vinculados
em Motores. Nenhum conjunto demonstrativo é carregado.

O perfil no avatar continua local, separado da conta administrativa da API.
No navegador com `npm run dev`, os bindings Go não estão disponíveis: a tela
informa a indisponibilidade, sem substituir o backend por dados fictícios.

Verificação: primeiro `npm run build` em `frontend/`; depois `go test -race ./...`
e `go vet ./...` nesta pasta. Não executar o build do frontend simultaneamente
aos testes Go: o embed do Wails lê o diretório `frontend/dist`.

Consulte o [README da raiz](../../README.md) e o [guia da API](../server/README.md).

## Logs do gateway

Clique no card do gateway no workflow para abrir o card de logs ao lado da topologia. Na aba **Gateways**, clique no card para expandir seus logs logo abaixo. O console usa a sessão da API já configurada e acompanha somente o gateway selecionado, com níveis DEBUG/INFO/WARN/ERROR, busca, pausa, rolagem automática, limpeza local e exportação `.log`.

A consulta ocorre a cada dois segundos, enquanto a janela estiver visível e o console não estiver pausado. O C3 envia lotes de logs a cada cinco segundos, então a visualização tem atraso de rede/lote. A primeira consulta carrega até 200 linhas recentes; a tela mantém até 2.000. Timestamp `+HH:MM:SS.mmm` é tempo desde o boot do gateway; o tooltip e a exportação incluem horário de chegada à API, sessão e sequência. Lacunas na sequência de logs são sinalizadas. A pausa/limpeza não altera medições nem apaga registros no servidor.

Requer API com `/api/v1/gateways/{id}/logs` e firmware C3 atualizado com `/api/v1/ingest/logs`. Firmware antigo só envia heartbeat e medições; nenhum log serial anterior é reconstruído ou simulado. Logs internos do S3 e mensagens do ROM não estão no transporte atual. Se a rede cair, só as linhas ainda disponíveis na fila RAM do C3 poderão chegar após reconectar.
