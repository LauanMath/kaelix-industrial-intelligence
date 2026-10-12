import {useEffect,useState} from 'react';
import {ConnectAPI,DisconnectAPI} from '../../wailsjs/go/main/App';
import type {Connection as ConnectionInfo} from '../lib/types';
export function Connection({info,onConnect,onDisconnect}: {info:ConnectionInfo|null;onConnect:(value:ConnectionInfo)=>void;onDisconnect:()=>void}){
 const [endpoint,setEndpoint]=useState(''),[email,setEmail]=useState(''),[password,setPassword]=useState(''),[error,setError]=useState(''),[busy,setBusy]=useState(false);
 useEffect(()=>{setEndpoint(info?.endpoint||localStorage.getItem('kaelix.endpoint')||'http://127.0.0.1:8080');setEmail(info?.email||'');},[info?.endpoint,info?.email]);
 return <section className="content-panel settings"><h2>Conexão com o servidor</h2><p className="subtle">Use uma conta da API para acessar cadastros e medições. O perfil local é independente dessa conta.</p>
 {info?.authenticated&&<div className="connection-account"><span className="status NORMAL">Sessão autenticada</span><p>{info.email}</p><small>Válida até {new Date(info.expiresAt).toLocaleString('pt-BR')}</small><button className="button" disabled={busy} onClick={async()=>{setBusy(true);setError('');try{await DisconnectAPI();onDisconnect()}catch(err){setError(String(err))}finally{setBusy(false)}}}>Sair da conta</button></div>}
 <form onSubmit={async e=>{e.preventDefault();setBusy(true);setError('');try{const result=await ConnectAPI(endpoint,email,password);setPassword('');onConnect(result)}catch(err){setError(String(err))}finally{setBusy(false)}}}>
 <label>Endereço da API<input type="url" required value={endpoint} onChange={e=>setEndpoint(e.target.value)} placeholder="https://api.exemplo.com"/></label>
 <label>E-mail da conta<input type="email" required autoComplete="username" value={email} onChange={e=>setEmail(e.target.value)}/></label>
 <label>Senha<input type="password" required maxLength={72} autoComplete="current-password" value={password} onChange={e=>setPassword(e.target.value)}/></label>
 <button className="button primary" disabled={busy} type="submit">{busy?'Conectando…':info?.authenticated?'Conectar novamente':'Conectar à API'}</button>
 {error&&<p className="notice" role="alert">{error}</p>}</form><p className="notice">A senha não é salva. A sessão fica no banco privado deste computador até expirar ou você sair. HTTP é permitido apenas para localhost ou IP privado da bancada.</p></section>
}
