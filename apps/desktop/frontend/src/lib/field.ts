// Types and helpers of the command and collection screens. docs/comando-e-coleta.md is the contract.
export interface Routine {days:number;start_min:number;end_min:number}
export interface DeviceConfig {version:number;cycle_interval_s:number;window_samples:number;channels:number;accel_range:number;gyro_range:number;dlpf:number;tx_power_dbm:number;burst_profile:number;wave_every:number;link_loss_s:number;checkin_interval_s:number;tz_offset_min:number;routine:Routine[]}
export type RadioState='pending'|'radiated'|'applied';
export interface DeviceConfigState {device_id:string;config:DeviceConfig|null;state:RadioState|null;radiated_version:number|null;reported_version:number|null;reported_command_seq:number|null;updated_at:string|null;radiated_at:string|null;reported_at:string|null;applied_at:string|null;defaults:DeviceConfig}
export interface ConfigIssue {path:string;reason:string;minimum?:number}
export interface ConfigCheck {issues:ConfigIssue[];cycle_seconds:number|null;minimum_cycle_s:number|null}
export interface DeviceCommand {device_id:string;seq:number;kind:string;state:RadioState;created_at:string;radiated_at:string|null;applied_at:string|null}
export interface WifiStatus {ssid:string;has_password:boolean}
export interface GatewayConfigView {version:number;wifi:WifiStatus[];heartbeat_interval_s:number;sync_interval_s:number;ap_when_offline:boolean;tx_power_dbm:number}
export interface GatewayConfigState {gateway_id:string;config:GatewayConfigView|null;state:RadioState|null;reported_version:number|null;updated_at:string|null;reported_at:string|null;applied_at:string|null;last_sync_at:string|null;defaults:GatewayConfigView}
export interface GatewayFirmware {id:string;gateway_id:string;version:string;sha256:string;size:number;state:string;reason:string;created_at:string;offered_at:string|null;downloaded_at:string|null;finished_at:string|null}
export interface GatewayFirmwareStatus {gateway_id:string;current_version:string|null;items:GatewayFirmware[]}
export interface LinkInfo {mac:string;firmware_version:string;gateway_id:string;paired:boolean;connected:boolean;ssid:string;ip:string;config_version:number}
export interface LinkNetwork {ssid:string;rssi:number;secure:boolean}
export interface Waveform {id:number;gateway_id:string;gateway_session_id:string;event_seq:number;device_id:string;device_sequence:number;boot_count:number;channels:number;window_samples:number;sample_rate_hz:number;accel_range:number;gyro_range:number;dlpf:number;burst_profile:number;fragment_total:number;fragments_missing:number[];ingested_at:string;estimated_received_at:string|null;queue_delay_ms:number;samples_b64?:string}
export interface WaveformPage {items:Waveform[];next_cursor?:string}
export interface Collection {id:number;name:string;device_id:string;condition:string;notes:string;started_at:string;ended_at:string|null}
export interface LinkRange {min:number;avg:number;max:number}
export interface CollectionSummary {collection:Collection;from:string;to:string;readings:number;readings_without_measurement:number;readings_saturated:number;readings_undated:number;sequence_gaps:number;sequence_missing:number;waveforms:number;waveforms_incomplete:number;waveforms_undated:number;fragments_total:number;fragments_missing:number;rssi_dbm:LinkRange|null;snr_db:LinkRange|null}
export interface ExportResult {folder:string;readings:number;windows:number;readingsUndated:number;windowsUndated:number}
export interface ExportProgress {stage:'readings'|'windows';done:number;total:number}
export interface EndpointProbe {endpoint:string;reachable:boolean;localEndpoint:string;localReachable:boolean;suggestLocal:boolean}

export const radioLabels:Record<RadioState,string>={pending:'Pendente',radiated:'Enviado pelo rádio',applied:'Aplicado'};
export const radioClass:Record<RadioState,string>={pending:'UNKNOWN',radiated:'ANOMALOUS',applied:'NORMAL'};
export const gatewayStateLabels:Record<string,string>={pending:'Pendente',applied:'Aplicado'};
export const commandLabels:Record<string,string>={reboot:'Reiniciar',factory_reset:'Restaurar padrões de fábrica',capture_now:'Capturar agora'};
export const firmwareLabels:Record<string,string>={pending:'Aguardando o gateway',offered:'Oferecido ao gateway',downloaded:'Baixado pelo gateway',applied:'Instalado',failed:'Falhou',superseded:'Substituído'};
export const channelNames=['ax','ay','az','gx','gy','gz'];
export const burstProfiles=[{code:0,label:'SF9, 125 kHz, 1,76 kbps'},{code:1,label:'SF7, 125 kHz, 5,47 kbps'},{code:2,label:'SF7, 250 kHz, 10,94 kbps'},{code:3,label:'SF7, 500 kHz, 21,88 kbps'}];
export const dayNames=['Dom','Seg','Ter','Qua','Qui','Sex','Sáb'];
const flagNames:[number,string][]=[[4,'Resposta automática ativa: o transmissor não ouve o gateway'],[8,'O transmissor rejeitou a última configuração'],[16,'Sem medição: fora da rotina'],[32,'Janela saturada']];
export function deviceWarnings(flags:number|null|undefined){return flagNames.filter(([bit])=>flags!==null&&flags!==undefined&&(flags&bit)!==0).map(([,label])=>label)}
export function minutesToTime(minutes:number){return `${String(Math.floor(minutes/60)).padStart(2,'0')}:${String(minutes%60).padStart(2,'0')}`}
export function timeToMinutes(value:string){const [h,m]=value.split(':').map(Number);return Number.isFinite(h)&&Number.isFinite(m)?h*60+m:0}
export function decimal(value:number,digits=2){return value.toLocaleString('pt-BR',{minimumFractionDigits:digits,maximumFractionDigits:digits}).replace('-','−')}
export function plain(value:number){return value.toLocaleString('pt-BR',{maximumFractionDigits:6}).replace('-','−')}
export function activeChannels(mask:number){return channelNames.map((name,bit)=>({name,bit})).filter(c=>(mask&(1<<c.bit))!==0)}
export function issueFor(issues:ConfigIssue[],path:string){return issues.filter(i=>i.path===path||i.path.startsWith(path+'[')||i.path.startsWith(path+'.')).map(i=>i.reason)}

// One array per channel, in physical units. Samples of a lost fragment arrive as zeros and
// come back null: they were never measured and must not be drawn as a signal at zero.
export function decodeWindow(w:Waveform){
 const channels=activeChannels(w.channels),binary=atob(w.samples_b64??''),view=new DataView(Uint8Array.from(binary,c=>c.charCodeAt(0)).buffer),frame=channels.length*2,missing=new Set(w.fragments_missing);
 const out=channels.map(c=>({...c,unit:c.bit<3?'g':'°/s',values:new Array<number|null>(w.window_samples).fill(null)}));
 for(let sample=0;sample<w.window_samples;sample++){if(missing.has(Math.floor(sample*frame/240)))continue;for(let i=0;i<channels.length;i++){const offset=(sample*channels.length+i)*2;if(offset+2>view.byteLength)continue;const raw=view.getInt16(offset,true);out[i].values[sample]=channels[i].bit<3?raw/(16384/2**w.accel_range):raw/(131/2**w.gyro_range)}}
 return out;
}
// Spans of samples without data, as [first,last] pairs.
export function missingSpans(w:Waveform){const frame=activeChannels(w.channels).length*2,per=240/frame;return [...w.fragments_missing].sort((a,b)=>a-b).map(index=>[index*per,Math.min((index+1)*per,w.window_samples)-1] as [number,number])}
// Between 3 and 7 round ticks that cover the range.
export function niceTicks(min:number,max:number){
 if(min===max){const pad=Math.abs(min)||1;min-=pad;max+=pad}
 const rough=(max-min)/5,power=10**Math.floor(Math.log10(rough)),step=[1,2,5,10].map(m=>m*power).find(s=>s>=rough)??power*10,start=Math.floor(min/step)*step,end=Math.ceil(max/step)*step,ticks:number[]=[];
 for(let t=start;t<=end+step/2;t+=step)ticks.push(Math.abs(t)<step/1e6?0:Number(t.toPrecision(12)));
 return ticks;
}
