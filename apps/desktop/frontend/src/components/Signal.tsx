import {useEffect,useMemo,useState} from 'react';
import {GetDeviceWaveforms,GetWaveform} from '../../wailsjs/go/main/App';
import type {Workspace} from '../lib/types';
import {stamp} from '../lib/types';
import {decimal,decodeWindow,missingSpans,niceTicks,plain,type Waveform,type WaveformPage} from '../lib/field';
import './Field.css';
export function Signal({workspace,initialDevice}:{workspace:Workspace;initialDevice?:string}){
 const [device,setDevice]=useState(initialDevice??workspace.devices[0]?.id??''),[items,setItems]=useState<Waveform[]>([]),[cursor,setCursor]=useState(''),[chosen,setChosen]=useState<number|null>(null),[detail,setDetail]=useState<Waveform|null>(null),[error,setError]=useState(''),[busy,setBusy]=useState(false);
 const load=async(next:string,keep:Waveform[])=>{setBusy(true);setError('');try{const page=await GetDeviceWaveforms(device,'','',next) as unknown as WaveformPage;setItems([...keep,...(page.items??[])]);setCursor(page.next_cursor??'')}catch(err){setError(String(err))}finally{setBusy(false)}};
 useEffect(()=>{setItems([]);setCursor('');setChosen(null);setDetail(null);if(device)void load('',[])},[device]);
 useEffect(()=>{if(chosen===null)return;let stale=false;setDetail(null);setError('');GetWaveform(chosen).then(w=>{if(!stale)setDetail(w as unknown as Waveform)}).catch(err=>{if(!stale)setError(String(err))});return()=>{stale=true}},[chosen]);
 if(!workspace.devices.length)return <section className="content-panel"><h2>Sinal</h2><p className="empty">Nenhum transmissor recebido ainda.</p></section>;
 return <main className="field-layout"><nav className="field-list" aria-label="Janelas de sinal"><div style={{padding:10}}><label className="field-hint">Transmissor<select style={{display:'block',width:'100%',marginTop:5,background:'var(--gray-950)',color:'#e0e0dd',border:'1px solid #4c4c4c',padding:5}} value={device} onChange={e=>setDevice(e.target.value)}>{workspace.devices.map(d=><option key={d.id} value={d.id}>{d.id}</option>)}</select></label></div>
 {items.map(w=><button key={w.id} className={chosen===w.id?'selected':''} aria-current={chosen===w.id?'true':undefined} onClick={()=>setChosen(w.id)}>{w.estimated_received_at?stamp(w.estimated_received_at):'Horário indisponível'}<small>{w.window_samples} amostras · sequência {w.device_sequence}</small>{w.fragments_missing.length>0&&<small>{w.fragments_missing.length} de {w.fragment_total} fragmentos ausentes</small>}</button>)}
 {!items.length&&!busy&&<p className="empty">Ainda não há janelas deste transmissor.</p>}{cursor&&<div style={{padding:10}}><button className="button" disabled={busy} onClick={()=>void load(cursor,items)}>Carregar anteriores</button></div>}</nav>
 <section className="field-main">{error&&<p className="field-warning" role="alert">{error}</p>}{detail?<WindowCharts window={detail}/>:chosen!==null&&!error?<p className="empty">Carregando a janela…</p>:<p className="empty">{items.length?'Escolha uma janela para ver cada canal.':'Sem sinal para mostrar.'}</p>}</section></main>;
}
function WindowCharts({window:w}:{window:Waveform}){
 const channels=useMemo(()=>decodeWindow(w),[w]),spans=useMemo(()=>missingSpans(w),[w]);
 return <><div className="field-head"><h2>Janela {w.id}</h2><span className="subtle">{w.estimated_received_at?stamp(w.estimated_received_at):'Horário indisponível'}</span></div>
 <div className="field-facts"><div><span>Amostras por canal</span><strong>{w.window_samples}</strong></div><div><span>Taxa de amostragem</span><strong>{decimal(w.sample_rate_hz,1)} Hz</strong></div><div><span>Faixas</span><strong>±{2*2**w.accel_range} g · ±{250*2**w.gyro_range} °/s</strong></div><div><span>Boot · sequência</span><strong>{w.boot_count} · {w.device_sequence}</strong></div><div><span>Fragmentos</span><strong>{w.fragment_total-w.fragments_missing.length} de {w.fragment_total}</strong></div></div>
 {w.fragments_missing.length>0&&<p className="field-warning">Faltaram {w.fragments_missing.length} fragmentos. As faixas cinzentas são trechos sem dado, não sinal em zero.</p>}
 <div className="signal-charts" style={{marginTop:12}}>{channels.map((channel,i)=><Chart key={channel.bit} letter={String.fromCharCode(97+i)} name={channel.name} unit={channel.unit} kind={channel.bit<3?'Aceleração':'Velocidade angular'} values={channel.values} rate={w.sample_rate_hz} spans={spans}/>)}</div>
 <p className="field-note">Valores recebidos do sensor, convertidos pela faixa informada na janela. O tempo parte do início da janela.</p></>;
}
function Chart({letter,name,unit,kind,values,rate,spans}:{letter:string;name:string;unit:string;kind:string;values:(number|null)[];rate:number;spans:[number,number][]}){
 const w=900,h=150,left=62,right=10,top=8,bottom=34,plotW=w-left-right,plotH=h-top-bottom;
 const known=values.filter((v):v is number=>v!==null);
 if(!known.length)return <p className="field-note">{name}: todos os fragmentos deste canal estão ausentes.</p>;
 const yTicks=niceTicks(Math.min(...known),Math.max(...known)),yMin=yTicks[0],yMax=yTicks[yTicks.length-1],totalMS=(values.length-1)/rate*1000,xTicks=niceTicks(0,totalMS).filter(t=>t<=totalMS);
 const x=(ms:number)=>left+ms/totalMS*plotW,y=(v:number)=>top+(yMax-v)/(yMax-yMin)*plotH;
 let path='',open=false;values.forEach((v,i)=>{if(v===null){open=false;return}path+=`${open?'L':'M'}${x(i/rate*1000).toFixed(1)},${y(v).toFixed(1)}`;open=true});
 return <svg className="signal-chart" viewBox={`0 0 ${w} ${h}`} role="img" aria-label={`${kind} do canal ${name} em ${unit}, ao longo da janela`}><text className="panel-letter" x={2} y={12}>{letter}</text>
 {spans.map(([first,last])=><rect key={first} className="gap" x={x(first/rate*1000)} y={top} width={Math.max(x(last/rate*1000)-x(first/rate*1000),1)} height={plotH}/>)}
 <path className="trace" d={path}/><path className="axis" d={`M${left},${top}V${top+plotH}H${left+plotW}`}/>
 {yTicks.map(t=><g key={t}><line className="axis" x1={left-4} x2={left} y1={y(t)} y2={y(t)}/><text x={left-7} y={y(t)+3} textAnchor="end">{plain(t)}</text></g>)}
 {xTicks.map(t=><g key={t}><line className="axis" x1={x(t)} x2={x(t)} y1={top+plotH} y2={top+plotH+4}/><text x={x(t)} y={top+plotH+15} textAnchor="middle">{plain(t)}</text></g>)}
 <text x={left+plotW/2} y={h-4} textAnchor="middle">Tempo, t (ms)</text><text x={14} y={top+plotH/2} textAnchor="middle" transform={`rotate(-90 14 ${top+plotH/2})`}>{name} ({unit})</text></svg>;
}
