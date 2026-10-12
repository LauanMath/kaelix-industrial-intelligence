import {useState} from 'react';
import {motors, stateLabels, technicalEvents} from '../lib/demo';
import {Trend} from './Trend';
import {Icon} from './Icon';
import './Dashboard.css';

export function Dashboard({onInspect,onEvents}: {onInspect:(id:string)=>void;onEvents:()=>void}) {
 const [period,setPeriod]=useState('1 hora');
 const counts={NORMAL:motors.filter(m=>m.state==='NORMAL').length,ATTENTION:motors.filter(m=>m.state==='ATTENTION').length,UNKNOWN:motors.filter(m=>m.state==='UNKNOWN').length};
 const readings=motors.filter(m=>m.rms!==null);
 const mean=readings.reduce((sum,m)=>sum+m.rms!,0)/readings.length;
 return <main className="dashboard">
  <div className="dashboard-heading"><div><span className="eyebrow">RESUMO DA UNIDADE</span><h2>Dashboard operacional</h2></div><span className="subtle">Dados demonstrativos · última recepção simulada às 14:32</span></div>
  <section className="dashboard-kpis" aria-label="Indicadores da unidade">
   <article><span>Equipamentos</span><strong>{motors.length.toString().padStart(2,'0')}</strong><small>{readings.length} com RMS disponível</small></article>
   <article><span>Estado normal</span><strong className="green">{counts.NORMAL.toString().padStart(2,'0')}</strong><small>Classificação recebida · demo</small></article>
   <article><span>Requer atenção</span><strong className="amber">{counts.ATTENTION.toString().padStart(2,'0')}</strong><small>Bomba de circulação · M-02</small></article>
   <article><span>Classificação indisponível</span><strong>{counts.UNKNOWN.toString().padStart(2,'0')}</strong><small>Exaustor industrial · M-04</small></article>
  </section>
  <div className="dashboard-grid">
   <section className="dashboard-panel dashboard-trends"><div className="panel-header"><h2>Vibração por equipamento <small>RMS · g</small></h2><select aria-label="Período dos gráficos" value={period} onChange={e=>setPeriod(e.target.value)}><option>1 hora</option><option>15 min</option></select></div><div className="dashboard-sparklines">{motors.map(m=><button key={m.id} className={`dashboard-motor ${m.state}`} onClick={()=>onInspect(m.id)} title={`Inspecionar ${m.name}`}><div><span>{m.id}</span><strong>{m.rms?.toFixed(4)??'—'} <small>g</small></strong></div><p>{m.name}</p><Trend values={period==='15 min'?m.history.slice(-6):m.history}/><small>{m.rms===null?'Lacuna nas leituras':stateLabels[m.state]}</small></button>)}</div><div className="dashboard-axis"><span>{period==='1 hora'?'13:32':'14:17'} → 14:32 · simulado</span><span>Média das últimas leituras: {mean.toFixed(4)} g</span></div></section>
   <section className="dashboard-panel"><div className="panel-header"><h2>Distribuição dos estados</h2></div><div className="dashboard-states"><div className="state-bar" aria-label={`${counts.NORMAL} normais, ${counts.ATTENTION} em atenção, ${counts.UNKNOWN} desconhecidos`}>{Object.entries(counts).map(([state,count])=><span key={state} className={state} style={{flex:count}}/>)}</div>{Object.entries(counts).map(([state,count])=><div className="state-row" key={state}><span><i className={`dot ${state}`}/>{stateLabels[state as keyof typeof counts]}</span><strong>{count}</strong><small>{Math.round(count/motors.length*100)}%</small></div>)}<p className="subtle">Classificação desconhecida não equivale a motor normal.</p></div><div className="dashboard-network"><Icon name="gateways"/><div><strong>GW-01 · Gateway da unidade</strong><p>Online · simulado</p></div><span className="status NORMAL">DEMO</span></div></section>
   <section className="dashboard-panel"><div className="panel-header"><h2>Temperatura e enlace</h2></div><div className="dashboard-table"><table><thead><tr><th>Motor</th><th>Temperatura</th><th>RSSI</th><th>Estado</th></tr></thead><tbody>{motors.map(m=><tr key={m.id}><td><button className="table-select" onClick={()=>onInspect(m.id)}>{m.id}</button></td><td>{m.temperature===null?'—':`${m.temperature.toFixed(1)} °C`}</td><td>{m.rssi} dBm</td><td><span className={`status ${m.state}`}>{stateLabels[m.state]}</span></td></tr>)}</tbody></table></div></section>
   <section className="dashboard-panel"><div className="panel-header"><h2>Atividade recente</h2><button className="text-button" onClick={onEvents}>Ver eventos <Icon name="arrow"/></button></div><div className="dashboard-activity">{technicalEvents.map(e=><article key={e.time}><div><i className={`dot ${e.kind}`}/><time>{e.time}</time><span>{e.motor}</span></div><h3>{e.title}</h3><p>{e.detail}</p></article>)}</div></section>
  </div>
 </main>;
}
