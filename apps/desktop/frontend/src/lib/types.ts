export type MotorState='NORMAL'|'ANOMALOUS'|'UNKNOWN';
export const stateLabels:Record<MotorState,string>={NORMAL:'Normal',ANOMALOUS:'Anomalia',UNKNOWN:'Desconhecido'};
export interface Reading {id:number;gateway_id:string;gateway_session_id:string;device_id:string;event_seq:number;device_sequence:number;state:MotorState;diag_code:number;rms_g:number|null;temperature_c:number|null;rssi_dbm:number;snr_db:number;gateway_received_at:string|null;ingested_at:string;estimated_received_at:string|null;queue_delay_ms:number}
export interface Device {id:string;profile:string;notes:string;motor_id:string|null;latest:Reading|null}
export interface RegisteredMotor {id:string;name:string;location:string;device_id:string|null;expected_interval_seconds:number;latest:Reading|null}
export interface Gateway {id:string;name:string;location:string;last_ingested_at:string|null;last_heartbeat_at:string|null;active_session_id:string|null;activity:'RECENT'|'STALE'|'UNKNOWN';heartbeat:{firmware_version:string;heartbeat_seq:number;uptime_ms:number;sent_at:string|null;health:{radio_initialized:boolean;wifi_connected:boolean;ntp_synchronized:boolean;queue_size:number;queue_capacity:number;free_heap_bytes:number;wifi_rssi_dbm:number|null};counters:{lora_valid:number;lora_rejected:number;spi_errors:number;http_failed_attempts:number;readings_acknowledged:number;queue_dropped:number}}|null}
export interface Workspace {motors:RegisteredMotor[];devices:Device[];gateways:Gateway[];recentReadings:Reading[];fetchedAt:string}
export interface Connection {endpoint:string;email:string;authenticated:boolean;expiresAt:string}
export interface HistoryPage {items:Reading[];next_cursor?:string}
export interface Equipment {id:string;deviceId:string|null;name:string;area:string;state:MotorState;rms:number|null;temperature:number|null;rssi:number|null;sequence:number|null;latest:Reading|null;expectedInterval:number;registered:boolean}
export function equipmentFrom(w:Workspace):Equipment[]{
 const result:Equipment[]=w.motors.map(m=>({id:m.id,deviceId:m.device_id,name:m.name,area:m.location||'Local não informado',state:m.latest?.state??'UNKNOWN',rms:m.latest?.rms_g??null,temperature:m.latest?.temperature_c??null,rssi:m.latest?.rssi_dbm??null,sequence:m.latest?.device_sequence??null,latest:m.latest,expectedInterval:m.expected_interval_seconds,registered:true}));
 const associated=new Set(w.motors.map(m=>m.device_id));
 for(const d of w.devices)if(!associated.has(d.id))result.push({id:d.id,deviceId:d.id,name:`Sensor ${d.id}`,area:'Não associado a um motor',state:d.latest?.state??'UNKNOWN',rms:d.latest?.rms_g??null,temperature:d.latest?.temperature_c??null,rssi:d.latest?.rssi_dbm??null,sequence:d.latest?.device_sequence??null,latest:d.latest,expectedInterval:0,registered:false});
 return result;
}
export function stamp(value:string|null|undefined){return value?new Date(value).toLocaleString('pt-BR'):'—'}
export function clock(value:string|null|undefined){return value?new Date(value).toLocaleTimeString('pt-BR'):'—'}
export function age(value:string|null|undefined,now=Date.now()){
 if(!value)return 'Não disponível';const seconds=Math.max(0,Math.floor((now-new Date(value).getTime())/1000));
 if(seconds<60)return `Há ${seconds} s`;if(seconds<3600)return `Há ${Math.floor(seconds/60)} min`;return `Há ${Math.floor(seconds/3600)} h`;
}
export function fresh(m:Equipment,now=Date.now()){
 if(m.expectedInterval<=0)return null;
 if(!m.latest)return false;
 if(!m.latest.estimated_received_at)return null;
 const seconds=(now-new Date(m.latest.estimated_received_at).getTime())/1000;
 // Unassociated sensors have no configured freshness threshold.
 return m.expectedInterval>0 ? seconds<=m.expectedInterval*3+5 : null;
}
export function exportCSV(rows:Reading[]){
 const fields=['device_id','gateway_id','gateway_session_id','event_seq','device_sequence','state','diag_code','rms_g','temperature_c','rssi_dbm','snr_db','gateway_received_at','estimated_received_at','ingested_at','queue_delay_ms'] as const;
 const cell=(v:unknown)=>`"${String(v??'').replace(/"/g,'""')}"`;
 const dates=new Set(['gateway_received_at','estimated_received_at','ingested_at']);
 const csv='\uFEFF'+[fields.join(';'),...rows.map(r=>fields.map(f=>cell(dates.has(f)&&r[f]?new Date(String(r[f])).toISOString():r[f])).join(';'))].join('\r\n');
 const url=URL.createObjectURL(new Blob([csv],{type:'text/csv;charset=utf-8'}));const link=document.createElement('a');link.href=url;const today=new Date(),date=[today.getFullYear(),String(today.getMonth()+1).padStart(2,'0'),String(today.getDate()).padStart(2,'0')].join('-');link.download=`kaelix-leituras-${date}.csv`;link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
}
