# Gateway HTTP v1

Contrato HTTP entre o gateway e a API. A fonte comum é a especificação
[Comando e coleta sem cabo](../../../docs/comando-e-coleta.md); uma mudança lá exige mudar
estes schemas, os DTOs em `packages/contracts`, a API e o firmware.

Os arquivos originais vieram de `kaelix-firmware/docs/contracts/gateway-v1`. Esta pasta os
estende com os campos opcionais da especificação: pacote v3 nas leituras, `previous_session`,
saúde e contadores novos no heartbeat, `minimum` nos erros, e os contratos novos de janelas
(`waveforms-request`) e sincronização (`sync-request`, `sync-response`). Um pacote v2 sem os
campos novos continua válido.

O [guia da API](../../../apps/server/README.md) documenta persistência, autenticação, execução
e a política de sessões de heartbeat do servidor.

Os exemplos são fixtures técnicas: não representam credenciais nem uma integração física
validada. Os testes da API conferem cada exemplo contra o seu schema.
