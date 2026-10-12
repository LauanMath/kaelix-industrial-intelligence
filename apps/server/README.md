# Kaelix API

API Go independente do desktop, com PostgreSQL, migrações transacionais e contrato
HTTP v1 compartilhado em `packages/contracts`. Usa `net/http` e pgx; continua
recebendo leituras com o Wails fechado.

## Executar com Docker Compose

Na raiz do repositório:

```sh
cp deploy/.env.example deploy/.env
# Preencha POSTGRES_PASSWORD, API_ADMIN_EMAIL e API_ADMIN_PASSWORD em deploy/.env.
docker compose --env-file deploy/.env -f deploy/compose.yaml up --build -d
curl --fail http://127.0.0.1:8080/readyz
```

O Compose cria API + PostgreSQL 18 com volume persistente. Por padrão, portas publicadas
em loopback: API 8080 e banco 5432. A API permite configurar outra interface
com `API_BIND_ADDRESS`; o banco permanece em loopback. A senha do banco deve ser aleatória e compatível
com URL (por exemplo, gerada por `openssl rand -hex 24`). Não versionar `.env`.
A imagem oficial do PostgreSQL 18 usa o volume em `/var/lib/postgresql`;
consulte a [documentação da imagem](https://hub.docker.com/_/postgres).

## Executar sem Docker

Requer Go 1.26.6+ e PostgreSQL 16+ acessível. Crie uma base dedicada e configure:

```sh
export DATABASE_URL='postgres://USER:PASSWORD@127.0.0.1:5432/kaelix?sslmode=disable'
export API_ADDR='127.0.0.1:8080'
export API_ADMIN_EMAIL='admin@example.com'
# Defina API_ADMIN_PASSWORD no ambiente: 12 a 72 bytes.
cd apps/server
go run ./cmd/api
```

`DATABASE_URL` é obrigatória. O processo verifica o banco e aplica as migrações
embutidas antes de escutar; instâncias simultâneas serializam migrações com um
advisory lock. SIGINT/SIGTERM encerra requisições em andamento antes de fechar o pool.
O binário pode ser construído com `go build -o api ./cmd/api`.

`API_ADMIN_EMAIL` e `API_ADMIN_PASSWORD` criam a conta administrativa inicial;
uma conta existente mantém sua senha. Ambos podem ser removidos do ambiente nos
próximos startups. A senha usa bcrypt; sessões aleatórias expiram em 24 horas e
somente seu SHA-256 é armazenado. O perfil SQLite do desktop permanece independente.
Esta versão tem acesso administrativo a uma instalação; não implementa organizações
ou permissões por local. Login permite até dez tentativas por minuto/IP/instância,
sem confiar em cabeçalhos encaminhados. Atrás de proxy, aplicar também limite de
login na borda para os IPs originais.

HTTP local serve ao desenvolvimento. Para internet, colocar a API atrás de um
proxy HTTPS e usar PostgreSQL com TLS (`sslmode=verify-full`). O gateway deve
validar o certificado do endpoint. Banco e credenciais não são distribuídos ao desktop.

## Endpoints

Todos os caminhos abaixo usam o prefixo `/api/v1`. Respostas usam JSON, exceto logout.
Métodos com corpo exigem `Content-Type: application/json`, limite de 64 KiB e
campos conhecidos. Nas rotas de ingestão, o limite é **32 KiB**, sem JSON comprimido.
Chaves duplicadas em qualquer profundidade e documentos extras são rejeitados.
O formato de erro é versionado e inclui `code`, `message`, `retryable` e `details`;
veja [error-response.schema.json](../../packages/contracts/gateway-v1/error-response.schema.json).

| Método | Caminho | Credencial | Resultado |
|---|---|---|---|
| POST | `/auth/login` | Pública | `access_token`, `token_type`, `expires_at` |
| POST | `/auth/logout` | Usuário | Revoga sessão; 204 |
| GET | `/readings/recent` | Usuário | Últimas 50 recepções, por chegada à API |
| GET / POST | `/devices` | Usuário | Lista / cria dispositivo |
| PUT | `/devices/{id}` | Usuário | Atualiza perfil e observações |
| GET | `/devices/{id}/readings` | Usuário | Histórico do sensor, mesmo sem motor |
| GET / POST | `/motors` | Usuário | Lista com última leitura / cria motor |
| PUT | `/motors/{id}` | Usuário | Atualiza cadastro e associação |
| GET | `/motors/{id}/readings` | Usuário | Histórico do sensor atualmente associado |
| GET / POST | `/gateways` | Usuário | Lista saúde / cria gateway e credencial |
| POST | `/gateways/{id}/rotate-token` | Usuário | Nova credencial; revoga anterior |
| POST | `/ingest/readings` | Gateway | Confirma lote persistido / duplicatas |
| POST | `/ingest/heartbeat` | Gateway | Persiste saúde e confirma recebimento |

`GET /healthz` verifica processo; `GET /readyz` também verifica banco (503 se indisponível).
Sessões de usuário e tokens de gateway são separados: um não acessa as rotas do outro.
Em ambos, enviar `Authorization: Bearer TOKEN`. Credencial de gateway é retornada
somente na criação/rotação e armazenada como SHA-256; não aparece nas listagens.

IDs de motor: 1 a 64 caracteres, início alfanumérico e demais alfanuméricos, `_`, `-`, `.`.
Gateway e sessão usam **UUIDs em minúsculas**. Cadastro de gateway sem `id` gera
UUID v4; um UUID explícito pode ser usado para importar cadastro/fixtures.
O firmware usa o UUID retornado pela API e gera outro UUID para cada boot. ID de dispositivo: oito caracteres hexadecimais
maiúsculos. Associação motor→dispositivo é única; `device_id: null` remove o vínculo.
Cadastro de motor cria o dispositivo se necessário. `PUT` substitui os campos do
cadastro (não é atualização parcial). Duplicação de cadastro/associação retorna 409.

### Fluxo de cadastro e ingestão

```sh
# Obtenha access_token e guarde-o em USER_TOKEN.
curl --fail-with-body http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  --data '{"email":"admin@example.com","password":"YOUR_ADMIN_PASSWORD"}'

# Obtenha gateway_token e guarde-o em GATEWAY_TOKEN.
curl --fail-with-body http://127.0.0.1:8080/api/v1/gateways \
  -H "Authorization: Bearer $USER_TOKEN" -H 'Content-Type: application/json' \
  --data '{"id":"5ef49647-9449-4f47-8d1d-c06611c3ba81","name":"Gateway da unidade","location":"Setor A"}'

curl --fail-with-body http://127.0.0.1:8080/api/v1/ingest/readings \
  -H "Authorization: Bearer $GATEWAY_TOKEN" -H 'Content-Type: application/json' \
  --data-binary @apps/server/testdata/readings-v1.json

curl --fail-with-body http://127.0.0.1:8080/api/v1/motors \
  -H "Authorization: Bearer $USER_TOKEN" -H 'Content-Type: application/json' \
  --data '{"id":"M-01","name":"Motor de alimentação","location":"Setor A","device_id":"D6F61B44","expected_interval_seconds":11}'

curl --fail-with-body 'http://127.0.0.1:8080/api/v1/motors/M-01/readings?limit=100' \
  -H "Authorization: Bearer $USER_TOKEN"
```

O contrato adotado é o do firmware C3: um snapshot sem alterações dos schemas e
exemplos está em [packages/contracts/gateway-v1](../../packages/contracts/gateway-v1).
`testdata/readings-v1.json` é a mesma fixture de leitura fornecida pelo firmware;
fixtures não comprovam integração física. A API não reconstrói o CRC a partir do
JSON: o C3 deve validar o quadro LoRa de 21 bytes antes de enviar.

Versões: JSON `schema_version: 1`, LoRa `packet_version: 2`. Estados: `NORMAL`,
`ANOMALOUS`, `UNKNOWN` (nenhum `ATTENTION` no protocolo). `device_sequence` e
`event_seq` usam uint32; `diag_code` aceita todo uint8, preservando códigos desconhecidos;
`packet_crc16` usa uint16. RMS finito não negativo; `null` somente com `UNKNOWN`.
Temperatura pode ser `null`; RSSI/SNR precisam ser finitos, sem faixas extras impostas
pelo servidor. Todos os campos são obrigatórios, inclusive os anuláveis.
Timestamps do gateway devem ser RFC3339 UTC terminados em `Z` ou `null`.

Um dispositivo recebido sem cadastro é criado automaticamente, com `motor_id: null`.
`PUT /devices/{id}` acrescenta `profile` e `notes` sem alterar as medições.
Sem medição disponível, `latest` é `null`. Não sintetizamos leituras ou estados normais.

### Confirmação e reenvio

Identidade de evento: `(gateway_id, gateway_session_id, event_seq)`. Um lote é gravado
em uma transação; a resposta só confirma após commit:

```json
{
  "schema_version": 1,
  "gateway_id": "5ef49647-9449-4f47-8d1d-c06611c3ba81",
  "gateway_session_id": "17162ec5-4fe1-4dac-a871-879120324a57",
  "server_received_at": "2026-10-11T19:00:00Z",
  "acknowledged": [{"event_seq": 1, "status": "stored"}]
}
```

No reenvio idêntico, o evento é confirmado com `status: duplicate`. Lotes podem
misturar novos eventos e duplicatas; cada evento recebe uma confirmação.
`upload_uptime_ms` pode mudar entre tentativas; todos os campos dentro da leitura
são imutáveis. Mesma identidade com conteúdo diferente retorna 409 `EVENT_CONFLICT`
e reverte o lote inteiro. A medição original não é sobrescrita.
`server_received_at` representa a tentativa atual; `ingested_at` é preservado.
A sequência do sensor pode repetir após reset; `event_seq` não repete dentro do lote.

Uptimes e contadores aceitam inteiros até `9007199254740991` (representação segura
no TypeScript). A recepção monotônica não pode superar o upload na mesma sessão.
`queue_delay_ms = upload_uptime_ms - gateway_rx_uptime_ms`; `ingested_at` usa UTC do
servidor. `estimated_received_at` usa o timestamp confiável do gateway ou uma
estimativa pela ingestão menos o atraso da fila; esse campo não é aquisição no motor.
A estimativa é `null` se um atraso extremo ultrapassa o intervalo de datas RFC3339;
o atraso original permanece armazenado, sem overflow nem data recente artificial.
Última leitura e histórico usam recepção estimada e ID, mantendo estimativas
indisponíveis no final. Um filtro por período não inclui datas indetermináveis.

### Erros para o C3

| HTTP | Código | Repetir? |
|---|---|---|
| 400 / 422 | `INVALID_REQUEST` | Não |
| 422 | `UNSUPPORTED_SCHEMA` | Não |
| 401 | `UNAUTHORIZED` | Não; configurar credencial |
| 403 | `FORBIDDEN` | Não; conferir identidade |
| 409 | `EVENT_CONFLICT` | Não; investigar lote |
| 413 | `PAYLOAD_TOO_LARGE` | Não; corrigir tamanho |
| 429 | `RATE_LIMITED` | Sim; respeitar `Retry-After` |
| 503 | `SERVICE_UNAVAILABLE` | Sim; aplicar atraso e jitter |

Erros não contêm confirmações. `details` sempre é um array, com caminho e motivo
quando a validação semântica identifica o campo. Falhas de rede ou respostas sem
JSON não autorizam o firmware a descartar eventos pendentes.

### Histórico

`from` (inclusivo) e `to` (exclusivo): RFC3339 com timezone. `limit`: 1–1000, padrão 100.
Sem período, consulta todos os dados disponíveis. Resposta:

```json
{"items":[],"next_cursor":"OPAQUE_CURSOR_IF_MORE_DATA"}
```

Ordem decrescente por `(estimated_received_at, id)`, com desempate estável.
Na próxima página, reutilizar período/limite e passar `cursor`. O cursor é vinculado
ao dispositivo; formato inválido retorna 400. `next_cursor` é omitido no final.
Paginação não congela um snapshot: novas medições posteriores à posição já visitada
ficam para uma nova consulta. Trocar o dispositivo de um motor muda o histórico
consultado por esse motor; histórico antigo continua disponível pelo ID do dispositivo.

### Heartbeat

A estrutura completa vem de
[heartbeat-request.schema.json](../../packages/contracts/gateway-v1/heartbeat-request.schema.json)
e [sua fixture](../../packages/contracts/gateway-v1/examples/heartbeat-request.json):

```json
{
  "schema_version": 1,
  "gateway_id": "5ef49647-9449-4f47-8d1d-c06611c3ba81",
  "gateway_session_id": "17162ec5-4fe1-4dac-a871-879120324a57",
  "heartbeat_seq": 1, "uptime_ms": 30000, "sent_at": null,
  "firmware_version": "0.1.0",
  "health": {
    "radio_initialized": true, "wifi_connected": true, "ntp_synchronized": false,
    "queue_size": 0, "queue_capacity": 64, "free_heap_bytes": 80000,
    "wifi_rssi_dbm": -60
  },
  "counters": {
    "lora_valid": 1, "lora_rejected": 0, "spi_errors": 0,
    "http_failed_attempts": 0, "readings_acknowledged": 1, "queue_dropped": 0
  }
}
```

A resposta ecoa versão, gateway, sessão e `heartbeat_seq`, e inclui
`server_received_at`. Snapshots são persistidos por `(gateway, sessão, heartbeat_seq)`.
Reenvio idêntico e snapshot de sequência menor são confirmados, mas não alteram
estado atual ou horário de atividade. Mesma identidade com corpo diferente gera
409. Um novo snapshot com uptime/contadores menores que o último da mesma sessão
gera 422; os contadores podem reiniciar em uma nova sessão.

O catálogo expõe `active_session_id`, `last_ingested_at`, `last_heartbeat_at`, o
heartbeat ativo e `activity`: `UNKNOWN` sem heartbeat, `RECENT` com atividade há
menos de 90 segundos, `STALE` depois disso. Conexão do gateway não classifica o motor.

**Política de sessão:** a primeira recepção (leitura ou heartbeat) registra cada
sessão. O primeiro heartbeat ativa sua sessão; uma sessão conhecida mais recente
pode substituí-la. A anterior fica aposentada e jamais volta a substituir a ativa
por envio atrasado, mesmo com sequência maior. Sessões e snapshots são preservados
separadamente. Bloqueio transacional por gateway serializa alterações concorrentes.

UUIDs aleatórios não codificam a ordem de boot. Uma sessão antiga **nunca antes
vista pelo servidor** não pode ser identificada como antiga somente pelo UUID;
a política usa a ordem da primeira recepção no servidor. Ordenação garantida nesse
caso exige evolução coordenada do contrato (contador persistente de boot ou
ativação explícita), não inferência pelo UUID. Conforme o contrato do C3, não
enviar backlog de heartbeat ao reconectar.

A migração `002_gateway_contract.sql` mantém cadastros e medições existentes;
snapshots do formato inicial de heartbeat são preservados em
`legacy_gateway_heartbeats` e deixam de representar atividade v1.

## Verificação

```sh
# Cluster descartável e isolado, exige initdb + pg_ctl disponíveis no PATH.
./scripts/test-server-postgres.sh

# Ou banco de teste existente; cada teste cria e remove seu próprio schema.
TEST_DATABASE_URL='postgres://...' ./scripts/test-server-postgres.sh

cd apps/server
go vet ./...
go build ./cmd/api
```

O script executa `go test -race -count=1 ./...` contra PostgreSQL real. Cobre login,
permissões, rotação/logout, cadastro, sensores não associados, nulos, reenvios,
concorrência, conflitos com rollback, paginação/período, sessões e heartbeats
atrasados, migrações e reabertura. Fixtures e respostas reais são verificadas
contra os JSON Schemas draft 2020-12 do firmware, com formatos habilitados;
limites uint32/safe integer, chaves duplicadas e corpo de 32 KiB são exercitados. Sem `TEST_DATABASE_URL`, `go test ./...` executa testes de validação e
marca testes de integração como skipped. O runner local foi validado com PostgreSQL 16.

## Próximas integrações

A API/banco central e o cliente do desktop estão implementados. O Wails consulta
cadastros, últimas leituras, histórico, saúde de gateways e recepções reais por
`/readings/recent`, com autenticação e atualização periódica. O fluxo C3 → API →
PostgreSQL está ativo na bancada. Eventos derivados (alertas, perda de sequência)
e publicação HTTPS pública permanecem em etapas separadas. Nenhum endpoint
`/events` é anunciado como pronto.

## Bancada C3 pela LAN neste Mac

A API nativa foi iniciada em `http://192.168.18.167:8080`, com o PostgreSQL local
em `127.0.0.1:5432`, base/role dedicados `kaelix_lan`. O banco permanece em loopback.
Verificar a disponibilidade:

```sh
curl --noproxy '*' --fail http://192.168.18.167:8080/readyz
```

Configuração e credenciais administrativas/de banco ficam em
`deploy/.env.lan.local` (ignorado, permissão 0600). O gateway foi cadastrado pela API;
seu ID e token foram salvos nesse arquivo e preenchidos diretamente em
`kaelix-firmware/include/gateway_config.local.h`, também ignorado e privado.
As configurações de Wi-Fi foram preservadas. No firmware: URL acima,
`KAELIX_ALLOW_HTTP_LAN=1`, CA vazia e credencial própria do gateway.
Nenhuma medição de teste foi inserida ao verificar o acesso.

O serviço desta sessão é gerenciado pelo launchd do usuário, com binário,
configuração de execução e logs em `.temp/lan-api`. Continua ativo após o terminal
fechar; não foi instalado para iniciar automaticamente após reboot/logout.
Na raiz deste repositório:

```sh
# Parar.
launchctl bootout "gui/$(id -u)/com.kaelix.api.lan"

# Iniciar novamente (após parar ou em uma nova sessão de login).
launchctl bootstrap "gui/$(id -u)" .temp/lan-api/com.kaelix.api.lan.plist
```

Se o IP do Mac mudar, atualizar `API_ADDR` em `deploy/.env.lan.local`, reiniciar o
serviço e atualizar a URL local do firmware. Mac e C3 precisam estar na mesma LAN;
a validação feita neste Mac não comprova conectividade física do C3. O firmware
precisa ser recompilado/gravado para incorporar o header alterado.

Para executar em primeiro plano usando a mesma configuração (com o serviço parado):

```sh
set -a
. deploy/.env.lan.local
set +a
cd apps/server
go run ./cmd/api
```

No Compose, a interface de publicação da API agora é configurável por
`API_BIND_ADDRESS` em `deploy/.env`, mantendo `127.0.0.1` como padrão. Para bancada,
definir `API_BIND_ADDRESS=192.168.18.167` e iniciar Compose com o serviço nativo
parado para liberar 8080. Esse Compose usa seu próprio banco/volume: cadastros e
credenciais da base nativa não migram automaticamente para o container.
HTTP LAN é exclusivo desta bancada; a implantação pela internet continua exigindo HTTPS.

## Logs remotos para o Dev mode

O contrato adicional está em [gateway-logs-v1](../../packages/contracts/gateway-logs-v1). `POST /api/v1/ingest/logs` exige o token do gateway e persiste lotes de até oito linhas de diagnóstico com deduplicação por gateway/sessão/sequência. `GET /api/v1/gateways/{id}/logs?after_id=...&limit=200` exige sessão de usuário e consulta por cursor incremental. Sem cursor, retorna a cauda mais recente. Retenção limitada a 5.000 linhas por gateway; a migração `004_gateway_logs.sql` cria a tabela separada. Logs não criam nem confirmam medições e não substituem o heartbeat como indicador de atividade.

Reiniciar a API com o binário atualizado aplica a migração. Firmware e desktop também precisam das novas versões; antes da gravação do C3, o console permanece sem linhas. Os testes com PostgreSQL real cobrem permissões, nulos/campos obrigatórios, duplicatas, conflitos com rollback, cursor e retenção; o frontend exibe mensagens como texto, sem HTML.
