import {useEffect,useRef,useState} from 'react';
import {SaveMotor} from '../../wailsjs/go/main/App';
import {contracts} from '../../wailsjs/go/models';
import type {Equipment,Workspace} from '../lib/types';
import {Icon} from './Icon';
export function MotorEditor({workspace,equipment,onClose,onSaved}:{workspace:Workspace;equipment:Equipment|null;onClose:()=>void;onSaved:()=>void}){
 const dialog=useRef<HTMLDialogElement>(null);const registered=workspace.motors.find(m=>m.id===equipment?.id);
 const [id,setId]=useState(registered?.id??''),[name,setName]=useState(registered?.name??''),[location,setLocation]=useState(registered?.location??''),[device,setDevice]=useState(equipment?.deviceId??''),[interval,setInterval]=useState(registered?.expected_interval_seconds??11),[error,setError]=useState(''),[busy,setBusy]=useState(false);
 useEffect(()=>{const previous=document.activeElement;dialog.current?.showModal();return()=>{if(previous instanceof HTMLElement)previous.focus()}},[]);
 return <dialog ref={dialog} className="motor-editor" aria-label={registered?'Editar motor':'Cadastrar motor'} onCancel={e=>{e.preventDefault();onClose()}}><header><h2>{registered?'Editar motor':'Cadastrar motor'}</h2><button className="button" aria-label="Fechar cadastro" onClick={onClose}><Icon name="close"/></button></header><form onSubmit={async e=>{e.preventDefault();setBusy(true);setError('');try{await SaveMotor(new contracts.MotorInput({id,name,location,device_id:device||null,expected_interval_seconds:interval}),!!registered);onSaved();onClose()}catch(err){setError(String(err))}finally{setBusy(false)}}}>
 <label>Identificador do motor<input required disabled={!!registered||busy} pattern="[A-Za-z0-9][A-Za-z0-9_.\-]{0,63}" maxLength={64} placeholder="M-01" value={id} onChange={e=>setId(e.target.value)}/></label>
 <label>Nome<input required maxLength={160} value={name} onChange={e=>setName(e.target.value)}/></label><label>Localização<input maxLength={256} value={location} onChange={e=>setLocation(e.target.value)}/></label>
 <label>Dispositivo<select value={device} onChange={e=>setDevice(e.target.value)}><option value="">Sem dispositivo associado</option>{workspace.devices.filter(d=>!d.motor_id||d.motor_id===registered?.id).map(d=><option key={d.id} value={d.id}>{d.id}{d.profile?` · ${d.profile}`:''}</option>)}</select></label>
 <label>Intervalo esperado de leitura · segundos<input type="number" min={1} max={86400} required value={interval} onChange={e=>setInterval(Number(e.target.value))}/></label>
 {error&&<p className="notice" role="alert">{error}</p>}<footer><button type="button" className="button" onClick={onClose}>Cancelar</button><button className="button primary" disabled={busy} type="submit">{busy?'Salvando…':'Salvar no servidor'}</button></footer></form></dialog>
}
