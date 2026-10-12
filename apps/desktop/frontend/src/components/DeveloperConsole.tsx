import {useEffect,useRef,useState} from 'react';
import {GetGatewayLogs} from '../../wailsjs/go/main/App';
import type {Gateway} from '../lib/types';
import './DeveloperConsole.css';
interface LogLine {id:string;gateway_id:string;gateway_session_id:string;log_seq:number;uptime_ms:number;level:string;event:string;message:string;truncated:boolean;received_at:string}
interface LogPage {items:LogLine[];next_cursor:string;has_more:boolean}
function uptime(ms:number){const h=Math.floor(ms/3600000),m=Math.floor(ms/60000)%60,s=Math.floor(ms/1000)%60;return `+${String(h).padStart(2,'0')}:${String(m).padStart(2,'0')}:${String(s).padStart(2,'0')}.${String(ms%1000).padStart(3,'0')}`}
function text(line:LogLine){return `[${uptime(line.uptime_ms)}][gateway][${line.level}][${line.event}] ${line.message}${line.truncated?' … [truncado]':''}`}
export function DeveloperConsole({gateway:selected,authenticated,endpoint,onClose}:{gateway:Gateway;authenticated:boolean;endpoint:string;onClose:()=>void}){
 const gateway=selected.id;
 const [lines,setLines]=useState<LogLine[]>([]),[paused,setPaused]=useState(false),[follow,setFollow]=useState(true),[level,setLevel]=useState('ALL'),[search,setSearch]=useState(''),[error,setError]=useState(''),[updated,setUpdated]=useState('');
 const cursor=useRef(''),screen=useRef<HTMLDivElement>(null);
 useEffect(()=>{cursor.current='';setLines([]);setError('');setUpdated('')},[gateway,endpoint,authenticated]);
 useEffect(()=>{
  if(!authenticated||!gateway||paused)return;
  let stopped=false;let timer:ReturnType<typeof setTimeout>;
  const poll=async()=>{
   if(document.hidden){timer=setTimeout(poll,2000);return}
   let delay=2000;
   try{
    const page=await GetGatewayLogs(gateway,cursor.current) as unknown as LogPage;
    if(!stopped){
     cursor.current=page.next_cursor;
     setLines(old=>{const ids=new Set(old.map(l=>l.id));return [...old,...page.items.filter(l=>!ids.has(l.id))].slice(-2000)});
     setError('');setUpdated(new Date().toLocaleTimeString('pt-BR'));
     if(page.has_more)delay=50;
    }
   }catch(err){if(!stopped)setError(String(err))}
   finally{if(!stopped)timer=setTimeout(poll,delay)}
  };
  void poll();return()=>{stopped=true;clearTimeout(timer)};
 },[authenticated,gateway,paused,endpoint]);
 useEffect(()=>{if(follow&&!paused&&screen.current)screen.current.scrollTop=screen.current.scrollHeight},[lines,follow,paused]);
 const visible=lines.filter(l=>(level==='ALL'||l.level===level)&&`${l.event} ${l.message} ${l.gateway_session_id}`.toLowerCase().includes(search.toLowerCase()));
 const exportLogs=()=>{
  const body=visible.map(l=>`${new Date(l.received_at).toISOString()} session=${l.gateway_session_id} seq=${l.log_seq} ${text(l)}`).join('\n');
  const url=URL.createObjectURL(new Blob([body],{type:'text/plain;charset=utf-8'}));const a=document.createElement('a');a.href=url;a.download=`kaelix-gateway-${gateway}.log`;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
 };
 const gapIDs=new Set(lines.filter((l,i)=>i>0&&l.gateway_session_id===lines[i-1].gateway_session_id&&l.log_seq>lines[i-1].log_seq+1).map(l=>l.id));
 return <section className="dev-console" aria-label="Console de diagnóstico remoto">
  <header><div><strong>LOGS DO GATEWAY</strong><span>{selected.name}</span></div><button className="button" onClick={onClose}>Fechar</button></header>
  <div className="dev-tools"><select aria-label="Severidade do log" value={level} onChange={e=>setLevel(e.target.value)}>{['ALL','DEBUG','INFO','WARN','ERROR'].map(l=><option key={l} value={l}>{l==='ALL'?'Todos os níveis':l}</option>)}</select><input aria-label="Filtrar logs" placeholder="Filtrar evento ou mensagem…" value={search} onChange={e=>setSearch(e.target.value)}/><button className="button" aria-pressed={paused} onClick={()=>setPaused(v=>!v)}>{paused?'Retomar':'Pausar'}</button><label className="dev-follow"><input type="checkbox" checked={follow} onChange={e=>setFollow(e.target.checked)}/> Acompanhar</label><button className="button" onClick={()=>setLines([])}>Limpar tela</button><button className="button" disabled={!visible.length} onClick={exportLogs}>Exportar .log</button></div>
  <div className="dev-status"><span>{!authenticated?'Conecte-se à API':paused?'Visualização pausada':error?'Consulta indisponível':'Atualização a cada 2 s'}{updated&&` · consulta ${updated}`}</span><span>Rádio: {selected?.heartbeat?selected.heartbeat.health.radio_initialized?'inicializado':'falha na inicialização':'sem informação'} · SPI: {selected?.heartbeat?.counters.spi_errors??'—'} · {visible.length} linhas</span></div>
  {error&&<p className="dev-error" role="alert">{error.includes('HTTP 404')?'A API precisa da atualização de logs remotos. Reinicie o servidor com a nova versão.':error}</p>}
  <div className="dev-output" ref={screen} role="log" aria-live="off">
   {!visible.length&&<p className="dev-empty">{!gateway?'Nenhum gateway disponível.':lines.length?'Nenhum log corresponde ao filtro.':'Aguardando logs enviados pelo C3. O gateway precisa do firmware com diagnóstico remoto.'}</p>}
   {visible.map(l=><div key={l.id} className={`dev-line dev-${l.level.toLowerCase()}`}><time title={`Recebido na API: ${new Date(l.received_at).toLocaleString('pt-BR')} · sessão ${l.gateway_session_id} · sequência ${l.log_seq}`}>{uptime(l.uptime_ms)}</time><span className="dev-level">{l.level}</span><span className="dev-event">[{l.event}]</span><span className="dev-message">{l.message}{l.truncated&&<em> … [truncado]</em>}{gapIDs.has(l.id)&&<em> · lacuna na sequência de logs</em>}</span></div>)}
  </div>
  <footer>Horário +HH:MM:SS é o tempo desde o boot do C3. Transporte em lotes a cada 5 s; logs podem chegar atrasados. Buffer de tela: 2.000 linhas. Logs internos do S3 não são transmitidos pelo pacote LoRa atual.</footer>
 </section>
}
