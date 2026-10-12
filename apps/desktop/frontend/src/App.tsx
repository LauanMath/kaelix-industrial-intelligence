import {useEffect, useState} from 'react';
import {GetProfile} from '../wailsjs/go/main/App';
import {store} from '../wailsjs/go/models';
import {EventsOn} from '../wailsjs/runtime/runtime';
import {Profile} from './components/Profile';
import {Dashboard} from './components/Dashboard';
import {Factory} from './components/Factory';
import {Trend} from './components/Trend';
import {motors, stateLabels, technicalEvents, exportCSV} from './lib/demo';
import {Brand} from './components/Brand';
import {Icon} from './components/Icon';
import './App.css';
import './brand.css';
const tabs=['Visão geral','Motores','Gateways','Eventos','Configurações','Dashboard'] as const;
type Tab=typeof tabs[number];
function App(){
 const [tab,setTab]=useState<Tab>('Visão geral'),[selected,setSelected]=useState('M-01'),[search,setSearch]=useState(''),[period,setPeriod]=useState('1 hora'),[endpoint,setEndpoint]=useState(localStorage.getItem('kaelix.endpoint') || 'http://localhost:8080'),[saved,setSaved]=useState(false),[zoom,setZoom]=useState(1);
 const [collapsed,setCollapsed]=useState(false);
 const [profileOpen,setProfileOpen]=useState(false);
 const [user,setUser]=useState<store.Profile|null>(null);
 const refreshProfile=()=>{GetProfile().then(setUser).catch(()=>{});};
 useEffect(refreshProfile,[]);
 const [viewReset,setViewReset]=useState(0);
 useEffect(()=>{
 const navigate=(index:number)=>{if(index===6){setProfileOpen(true);return;}if(tabs[index])setTab(tabs[index]);};
 const onKey=(e:KeyboardEvent)=>{if(document.querySelector('dialog[open]'))return;if((e.metaKey||e.ctrlKey)&&/^[1-7]$/.test(e.key)){e.preventDefault();navigate(Number(e.key)-1);}if(e.key==='Escape')setSearch('');};
 window.addEventListener('keydown',onKey);
 const off=(window as unknown as {runtime?:unknown}).runtime?EventsOn('workspace:select',navigate):()=>{};
 return ()=>{window.removeEventListener('keydown',onKey);off();};
 },[]);
 const motor=motors.find(m=>m.id===selected)!;
 const filtered=motors.filter(m=>`${m.id} ${m.name} ${m.area}`.toLowerCase().includes(search.toLowerCase()));
 const history=period==='15 min'?motor.history.slice(-6):motor.history;
 return <div className={`app-shell ${collapsed?'sidebar-collapsed':''}`}>
 <header className="window-bar"><span className="window-title">Kaelix <span>— Unidade industrial</span></span><div className="window-account"><button className="topbar-avatar" onClick={()=>setProfileOpen(true)} aria-label="Abrir configurações do usuário" title={user?.name||'Perfil do usuário'}>{user?.avatarData?<img src={user.avatarData} alt=""/>:user?.name.trim().charAt(0).toUpperCase()||<Icon name="profile"/>}</button></div></header>
 <aside className="app-sidebar"><div className="sidebar-brand"><Brand compact={collapsed}/><button className="collapse-button" title={collapsed?'Expandir navegação':'Recolher navegação'} aria-label={collapsed?'Expandir navegação':'Recolher navegação'} onClick={()=>setCollapsed(v=>!v)}><Icon name="sidebar"/></button></div>
 {!collapsed&&<div className="sidebar-unit"><span className="dot NORMAL"/> Unidade industrial<small>Setor A · 6 equipamentos</small></div>}
 <nav aria-label="Áreas de trabalho">{tabs.map((t,i)=><button key={t} title={`${t} · Ctrl/⌘ ${i+1}`} className={tab===t?'nav active':'nav'} onClick={()=>setTab(t)} aria-current={tab===t?'page':undefined}><Icon name={(['monitor','motors','gateways','events','settings','dashboard','profile'] as const)[i]}/>{!collapsed&&<><span className="nav-label">{t==='Visão geral'?'Monitoramento':t}</span><kbd>{i+1}</kbd></>}</button>)}</nav>
 <div className="sidebar-bottom">{!collapsed&&<><span className="eyebrow">AMBIENTE LOCAL</span><p>Dados da unidade</p><small>API não conectada</small></>}</div></aside>
 <div className="workspace-bar"><div className="workspace-path"><span>Unidade industrial</span><span>›</span><h1>{tab==='Visão geral'?'Monitoramento':tab}</h1></div><div className="toolbar"><button className="button" onClick={()=>exportCSV(filtered)} title="Exportar últimas leituras"><Icon name="export"/> Exportar CSV</button></div></div>
 <div className={`work-area work-${tab==='Visão geral'?'monitor':tab==='Motores'?'motors':'other'}`}>
 {tab==='Dashboard'?<Dashboard onInspect={id=>{setSelected(id);setTab('Motores');}} onEvents={()=>setTab('Eventos')}/>:tab==='Visão geral'||tab==='Motores'?<>
 <div className="stats"><div><span>Motores monitorados</span><strong>06 <small>equipamentos</small></strong><p>01 gateway cadastrado</p></div><div><span>Estado normal</span><strong className="green">04 <small>motores</small></strong><p><i className="dot NORMAL"/> Classificação do motor</p></div><div><span>Requer atenção</span><strong className="amber">01 <small>motor</small></strong><p><i className="dot ATTENTION"/> Verificar M-02</p></div><div><span>Sem leitura recente</span><strong>01 <small>sensor</small></strong><p><i className="dot UNKNOWN"/> Comunicação indisponível</p></div></div>
 <main className={tab==='Motores'?'monitor-layout list-mode':'monitor-layout'}>
 <section className="plant-panel"><div className="panel-header"><div><span className="eyebrow">{tab==='Motores'?'EQUIPAMENTOS':'TOPOLOGIA DA UNIDADE'}</span><h2>{tab==='Motores'?'Motores da unidade':'Workflow de monitoramento'}</h2></div><span className="subtle">{tab==='Motores'?'06 motores':'Canvas 2D'}</span></div>
 {tab==='Motores'?<div className="motor-table"><label className="search"><Icon name="search"/> <input placeholder="Buscar motor, ID ou setor" value={search} onChange={e=>setSearch(e.target.value)}/></label><table><thead><tr><th>Equipamento</th><th>Estado</th><th>RMS · g</th><th>Temp.</th></tr></thead><tbody>{filtered.map(m=><tr key={m.id} className={selected===m.id?'selected':''} onClick={()=>setSelected(m.id)}><td><button className="table-select" onClick={()=>setSelected(m.id)}>{m.name}<small>{m.id} · {m.area}</small></button></td><td><span className={`status ${m.state}`}>{stateLabels[m.state]}</span></td><td>{m.rms?.toFixed(4)??'—'}</td><td>{m.temperature===null?'—':`${m.temperature} °C`}</td></tr>)}</tbody></table>{filtered.length===0&&<p className="empty">Nenhum motor encontrado.</p>}</div>:<><div className="plant-viewport"><div style={{transform:`scale(${zoom})`}} className="plant-transform"><Factory motors={motors} selected={selected} onSelect={setSelected} resetToken={viewReset}/></div><div className="map-caption"><span className="live-dot"/> TOPOLOGIA DA UNIDADE<small>Arraste os cabeçalhos · arraste o fundo para navegar</small></div><div className="zoom"><button aria-label="Aumentar zoom" disabled={zoom>=1.4} onClick={()=>setZoom(z=>Math.min(1.4,z+.1))}><Icon name="plus"/></button><button aria-label="Diminuir zoom" disabled={zoom<=.8} onClick={()=>setZoom(z=>Math.max(.8,z-.1))}><Icon name="minus"/></button><button aria-label="Restaurar zoom" onClick={()=>{setZoom(1);setViewReset(v=>v+1);}}><Icon name="reset"/></button></div></div><div className="plant-footer"><div className="legend"><span><i className="dot NORMAL"/> Normal</span><span><i className="dot ATTENTION"/> Atenção</span><span><i className="dot UNKNOWN"/> Desconhecido</span></div><span>LoRa → Gateway → API</span></div></>}
 </section>
 <aside className="telemetry"><div className="panel-header"><span className="eyebrow">TELEMETRIA DO MOTOR</span><span className={`status ${motor.state}`}>{stateLabels[motor.state]}</span></div><span className="motor-id">{motor.id} <span> / </span> {motor.area}</span><h2>{motor.name}</h2><p className="device">Dispositivo {motor.deviceId}</p><div className="reading"><span>Vibração RMS</span><strong>{motor.rms?.toFixed(4)??'—'} <small>g</small></strong><p>Aceleração · dados da unidade</p></div><div className="chart-heading"><span>Histórico de vibração</span><select aria-label="Período do histórico" value={period} onChange={e=>setPeriod(e.target.value)}><option>1 hora</option><option>15 min</option></select></div><div className={`chart ${motor.state}`}><Trend values={history} large/><div className="chart-labels"><span>{period==='1 hora'?'13:32':'14:17'}</span><span>14:32</span></div></div><div className="details"><div><span>Temperatura</span><strong>{motor.temperature??'—'} <small>°C</small></strong></div><div><span>Sinal LoRa</span><strong>{motor.rssi} <small>dBm</small></strong></div><div><span>Sequência</span><strong>{motor.sequence}</strong></div><div><span>Última recepção</span><strong>{motor.state==='UNKNOWN'?'Há 3 min':'Há 8 s'}</strong></div></div><div className="notice"><Icon name="info"/> {motor.state==='UNKNOWN'?'Classificação indisponível. Ausência de leitura não indica defeito no motor.':'Os indicadores desta tela são exemplos. A integração com a API será implementada em uma próxima etapa.'}</div></aside>
 </main>
 <section className="events-strip"><div><span className="eyebrow">ATIVIDADE DA UNIDADE</span><h2>Últimos eventos</h2><button className="text-button" onClick={()=>setTab('Eventos')}>Ver todos <Icon name="arrow"/></button></div>{technicalEvents.map(e=><article key={e.time}><span className="event-time"><i className={`dot ${e.kind}`}/>{e.time} <b>{e.motor}</b></span><h3>{e.title}</h3><p>{e.detail}</p></article>)}</section>
 </>:tab==='Gateways'?<section className="content-panel"><div className="panel-header"><h2>Infraestrutura de recepção</h2></div><div className="gateway-card"><span className="gateway-icon"><Icon name="gateways"/></span><div><h2>GW-01 · Gateway da unidade</h2><p>ESP32-C3 + RA-02 · Setor A</p></div><span className="status NORMAL">Online</span></div><div className="details"><div><span>Fila em RAM</span><strong>0 / 64</strong></div><div><span>Último heartbeat</span><strong>Há 5 s</strong></div><div><span>Descartes</span><strong>0</strong></div><div><span>Sessão</span><strong>—</strong></div></div><p className="notice">Não há comunicação ativa com um gateway.</p></section>:tab==='Eventos'?<section className="content-panel"><h2>Eventos técnicos</h2><p className="subtle">Registro de comunicação e classificação.</p>{technicalEvents.map(e=><div className="event-row" key={e.time}><i className={`dot ${e.kind}`}/><time>{e.time}</time><div><h3>{e.title}</h3><p>{e.detail}</p></div><span className="motor-id">{e.motor}</span></div>)}</section>:<section className="content-panel settings"><h2>Conexão com o servidor</h2><p className="subtle">Prepare o endereço da API para a próxima etapa de integração.</p><form onSubmit={e=>{e.preventDefault();localStorage.setItem('kaelix.endpoint',endpoint);setSaved(true)}}><label>Endereço da API<input type="url" required value={endpoint} onChange={e=>{setEndpoint(e.target.value);setSaved(false)}} placeholder="https://api.exemplo.com"/></label><button className="button primary" type="submit">Salvar endereço localmente</button>{saved&&<p role="status" className="green">Endereço salvo. A conexão ainda não está implementada.</p>}</form><div className="notice">Autenticação, coleta e persistência ainda não estão implementadas.</div></section>}
 </div>
 {profileOpen&&<Profile onClose={()=>setProfileOpen(false)} onProfileSaved={refreshProfile}/>}
 <footer className="app-footer"><span className="footer-brand"><img src="/brand/kaelix-mark.svg" alt=""/> KAELIX <span className="subtle">/ Desktop 0.1.0</span></span><span>Central industrial <span className="divider">·</span> Sem conexão com hardware</span></footer>
 </div>
}
export default App;
