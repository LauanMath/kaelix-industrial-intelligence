export type MotorState = 'NORMAL' | 'ATTENTION' | 'UNKNOWN';
export interface Motor { id: string; deviceId: string; name: string; area: string; state: MotorState; rms: number | null; temperature: number | null; rssi: number; sequence: number; history: (number | null)[] }
export const stateLabels: Record<MotorState, string> = { NORMAL: 'Normal', ATTENTION: 'Atenção', UNKNOWN: 'Desconhecido' };
export const motors: Motor[] = [
 {id:'M-01', deviceId:'D6F61B44', name:'Motor de alimentação', area:'Linha de montagem', state:'NORMAL', rms:0.0036, temperature:32.4, rssi:-55, sequence:1482, history:[.0031,.0033,.0032,.0035,.0033,.0036,.0034,.0036,.0035,.0034,.0036,.0036]},
 {id:'M-02', deviceId:'A1B2C301', name:'Bomba de circulação', area:'Sistema hidráulico', state:'ATTENTION', rms:0.0082, temperature:46.8, rssi:-61, sequence:932, history:[.0041,.0046,.0043,.005,.0058,.0055,.0062,.007,.0068,.0078,.008,.0082]},
 {id:'M-03', deviceId:'A1B2C302', name:'Motor da esteira', area:'Linha de montagem', state:'NORMAL', rms:0.0028, temperature:30.1, rssi:-58, sequence:1103, history:[.0025,.0026,.0024,.0028,.0027,.0026,.0028,.0027,.0028,.0026,.0027,.0028]},
 {id:'M-04', deviceId:'A1B2C303', name:'Exaustor industrial', area:'Ventilação', state:'UNKNOWN', rms:null, temperature:null, rssi:-72, sequence:628, history:[.003,.0031,.0032,.003,.0031,null,null,null,null,null,null,null]},
 {id:'M-05', deviceId:'A1B2C304', name:'Compressor auxiliar', area:'Utilidades', state:'NORMAL', rms:0.0041, temperature:35.6, rssi:-63, sequence:807, history:[.0038,.004,.0039,.0041,.004,.0038,.0042,.004,.0041,.004,.0042,.0041]},
 {id:'M-06', deviceId:'A1B2C305', name:'Motor de acabamento', area:'Linha de montagem', state:'NORMAL', rms:0.0032, temperature:31.7, rssi:-57, sequence:1421, history:[.003,.0031,.003,.0032,.0031,.0033,.0032,.0031,.0032,.003,.0031,.0032]},
];
export const technicalEvents = [
 {time:'14:32:08', motor:'M-02', title:'Classificação de atenção recebida', detail:'Bomba de circulação · estado demonstrativo', kind:'ATTENTION'},
 {time:'14:30:42', motor:'M-04', title:'Sensor sem leitura recente', detail:'Exaustor industrial · verificar comunicação', kind:'UNKNOWN'},
 {time:'14:28:11', motor:'GW-01', title:'Gateway conectado', detail:'Conexão restabelecida · fila esvaziada', kind:'NORMAL'},
];
export function exportCSV(items: Motor[]) {
 const rows = [['motor','dispositivo','estado','rms_g','temperatura_c','modo'], ...items.map(m => [m.id,m.deviceId,m.state,m.rms ?? '',m.temperature ?? '', 'demo'])];
 const blob = new Blob(['\uFEFF'+rows.map(r=>r.join(';')).join('\n')], {type:'text/csv;charset=utf-8'});
 const url=URL.createObjectURL(blob); const a=document.createElement('a'); a.href=url; a.download='kaelix-demo.csv'; a.click(); URL.revokeObjectURL(url);
}
