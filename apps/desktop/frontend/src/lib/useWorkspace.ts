import {useEffect,useRef,useState} from 'react';
import {GetConnection,GetWorkspace,GetDeviceHistory} from '../../wailsjs/go/main/App';
import type {Connection,Workspace,HistoryPage,Reading} from './types';
export function useWorkspace(){
 const [connection,setConnection]=useState<Connection|null>(null),[data,setData]=useState<Workspace|null>(null),[error,setError]=useState(''),[loading,setLoading]=useState(true),[revision,setRevision]=useState(0);
 useEffect(()=>{GetConnection().then(c=>setConnection(c)).catch(()=>setError('Backend desktop indisponível. Execute o aplicativo Wails.')).finally(()=>setLoading(false));},[]);
 const authenticated=!!connection?.authenticated;
 useEffect(()=>{
  if(!authenticated)return;let stopped=false;let timer:ReturnType<typeof setTimeout>;
  const poll=async()=>{
   if(document.hidden){timer=setTimeout(poll,2000);return}
   try{const result=await GetWorkspace();if(!stopped){setData(result as unknown as Workspace);setError('');}}
   catch(err){if(!stopped){setError(String(err));try{const c=await GetConnection();if(!stopped)setConnection(c)}catch{/* Keep the transport error visible until the runtime is available. */}}}
   finally{if(!stopped){setLoading(false);timer=setTimeout(poll,2000);}}
  };
  setLoading(true);void poll();return()=>{stopped=true;clearTimeout(timer)};
 },[authenticated,connection?.endpoint,revision]);
 const connected=(c:Connection)=>{setData(null);setError('');setConnection(c);setRevision(v=>v+1)};
 const disconnected=()=>{setConnection(c=>c?{...c,authenticated:false}:c);setData(null);setError('')};
 return {connection,data,error,loading,connected,disconnected,refresh:()=>setRevision(v=>v+1)};
}
export function useDeviceHistory(device:string|null,minutes:number,latest:number|undefined,authenticated:boolean){
 const [items,setItems]=useState<Reading[]>([]),[error,setError]=useState(''),[busy,setBusy]=useState(false),[cursor,setCursor]=useState('');
 const bounds=useRef({from:'',to:''});const generation=useRef(0);
 useEffect(()=>{
  const version=++generation.current;setItems([]);setCursor('');setError('');
  if(!device || !authenticated){setBusy(false);return}
  const to=new Date(),from=new Date(to.getTime()-minutes*60000);bounds.current={from:from.toISOString(),to:to.toISOString()};
  setBusy(true);GetDeviceHistory(device,bounds.current.from,bounds.current.to,'').then(raw=>{if(generation.current===version){const page=raw as unknown as HistoryPage;setItems(page.items);setCursor(page.next_cursor??'');}}).catch(err=>{if(generation.current===version)setError(String(err))}).finally(()=>{if(generation.current===version)setBusy(false)});
  return()=>{generation.current++};
 },[device,minutes,latest,authenticated]);
 const more=async()=>{if(!device||!cursor||busy)return;const version=generation.current;setBusy(true);try{const page=await GetDeviceHistory(device,bounds.current.from,bounds.current.to,cursor) as unknown as HistoryPage;if(version===generation.current){setItems(old=>[...old,...page.items.filter(r=>!old.some(existing=>existing.id===r.id))]);setCursor(page.next_cursor??'');setError('');}}catch(err){if(version===generation.current)setError(String(err))}finally{if(version===generation.current)setBusy(false)}};
 return {items,error,busy,cursor,more};
}
