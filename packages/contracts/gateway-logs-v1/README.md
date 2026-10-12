# Diagnóstico remoto do gateway — v1

`POST /api/v1/ingest/logs`: credencial Bearer do gateway, lote de 1–8 linhas. Versão, gateway e sessão seguem as identidades do transporte de medições. Cada linha contém `log_seq` (uint32 crescente por boot), `uptime_ms`, `level`, `event`, `message` e `truncated`. Mensagem limitada a 383 bytes UTF-8, sem NUL; o schema limita caracteres, enquanto a API também confere bytes. Todos os campos são obrigatórios. Corpo máximo 32 KiB.

Confirmação HTTP 200 ecoa versão/identidades, `server_received_at` UTC e `acknowledged`, array das sequências únicas do lote. A confirmação acontece após commit; reenvio idêntico não duplica. Mesma identidade com conteúdo diferente retorna 409 e reverte o lote. Diagnóstico não confirma nem modifica medições.

`GET /api/v1/gateways/{id}/logs?after_id=...&limit=200`: sessão de usuário da API. Retorna `items`, `next_cursor` e `has_more`. Sem cursor (ou zero), entrega a cauda mais recente em ordem crescente de ingestão. Com cursor decimal, entrega os próximos registros; IDs/cursor são strings para preservar bigint no TypeScript. `limit`: 1–500. Gateway inexistente retorna 404. Cursor é uma posição por gateway; o desktop reinicia a posição ao trocar o gateway/API.

Retenção: até 5.000 linhas por gateway no PostgreSQL. C3 mantém 64 linhas em RAM, envia até oito a cada cinco segundos e espaça retentativas até 60 segundos. Fila cheia perde a linha nova e deixa lacunas em `log_seq`. Reboot perde a fila RAM e cria nova sessão. Erros permanentes descartam somente o lote de logs; a fila de medições segue sua política própria. Não há replay dos logs anteriores ao firmware com esse recurso.

As linhas capturadas são as do logger Kaelix no C3 (BOOT, RADIO, HEALTH, RX, SPI, HTTP, UPLOAD etc.). Logs do ROM, logs internos da biblioteca e logs internos do S3 não estão nesse transporte. Horário monotônico não é aquisição no motor nem horário UTC; `received_at` é chegada à API. Credenciais nunca devem entrar nas mensagens.

Dev mode no desktop habilita a visualização local; o transporte do firmware continua ativo com o painel fechado. Painel consulta a cada dois segundos, exibe até 2.000 linhas, pode pausar, filtrar e exportar. Pausar/limpar afeta só a tela, não o servidor nem o dispositivo.
