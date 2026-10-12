import {useEffect, useRef, useState, type PointerEvent} from 'react';
import {Motor, stateLabels} from '../lib/demo';
import {Icon} from './Icon';
import './Workflow.css';

const positions = [[45,90],[355,90],[45,320],[355,320],[45,550],[355,550]];
const colors = {NORMAL:'#91d7a5', ATTENTION:'#dfb37a', UNKNOWN:'#858585'};
function MotorGlyph(){
 return <svg viewBox="0 0 64 48" fill="none" aria-hidden="true"><rect x="13" y="13" width="33" height="24" rx="5" fill="#353535" stroke="#aaa" strokeWidth="1.5"/><path d="M20 15v20m6-20v20m6-20v20m6-20v20" stroke="#7a7a7a"/><path d="M46 21h10v8H46M8 19h5v12H8M18 37v6h24v-6M22 13V7h15v6" stroke="#aaa" strokeWidth="1.5"/><path d="M13 43h34M25 7V4h9v3" stroke="#777" strokeWidth="1.5"/></svg>;
}

/** Drag node headers to rearrange topology; drag background to pan. */
export function Factory({motors, selected, onSelect, resetToken=0}: {motors:Motor[];selected:string;onSelect:(id:string)=>void;resetToken?:number}) {
 const [pan,setPan]=useState({x:0,y:0});
 const [nodes,setNodes]=useState(positions);
 const [gateway,setGateway]=useState([800,320]);
 const svg=useRef<SVGSVGElement>(null);
 const drag=useRef<{x:number;y:number;startX:number;startY:number;node:number|null}|null>(null);
 const point=(x:number,y:number)=>{const matrix=svg.current?.getScreenCTM();return matrix?new DOMPoint(x,y).matrixTransform(matrix.inverse()):new DOMPoint(x,y);};
 const begin=(e:PointerEvent,node:number|null)=>{
  if(e.button!==0)return;
  e.preventDefault();e.stopPropagation();
  const pt=point(e.clientX,e.clientY);
  const pos=node===null?[pan.x,pan.y]:node===-1?gateway:nodes[node];
  drag.current={x:pt.x,y:pt.y,startX:pos[0],startY:pos[1],node};
  svg.current?.setPointerCapture(e.pointerId);
  if(node!==null&&node>=0)onSelect(motors[node].id);
 };
 useEffect(()=>{setPan({x:0,y:0});},[resetToken]);
 return <svg ref={svg} className="factory workflow-canvas" viewBox="0 0 1120 820" role="group" aria-label="Workflow de moteurs. Arraste o cabeçalho dos cards para movê-los." onPointerDown={e=>{
  if(!(e.target as Element).closest('.workflow-card'))begin(e,null);
 }} onPointerMove={e=>{
  const d=drag.current;if(!d)return;
  const pt=point(e.clientX,e.clientY),x=d.startX+pt.x-d.x,y=d.startY+pt.y-d.y;
  if(d.node===null)setPan({x,y});
  else if(d.node===-1)setGateway([x,y]);
  else setNodes(current=>current.map((pos,i)=>i===d.node?[x,y]:pos));
 }} onPointerUp={()=>{drag.current=null}} onPointerCancel={()=>{drag.current=null}} onLostPointerCapture={()=>{drag.current=null}}>
  <g transform={`translate(${pan.x} ${pan.y})`}>
   <text x="45" y="60" fill="#929292" fontSize="10" letterSpacing="2">EQUIPAMENTOS / SENSORES</text>
   <text x={gateway[0]} y={gateway[1]-25} fill="#929292" fontSize="10" letterSpacing="2">RECEPÇÃO / GATEWAY</text>
   {motors.map((m,i)=>{const [x,y]=nodes[i],endX=gateway[0],endY=gateway[1]+45+i*24;return <g key={m.id}>
    <path d={`M${x+245} ${y+95}C${x+245+Math.max(65,Math.abs(endX-x-245)*.45)} ${y+95} ${endX-Math.max(65,Math.abs(endX-x-245)*.45)} ${endY} ${endX} ${endY}`} fill="none" stroke={m.id===selected?'#fb5102':m.state==='UNKNOWN'?'#484848':'#646464'} strokeWidth={m.id===selected?2:1.3} strokeDasharray={m.state==='UNKNOWN'?'5 6':undefined}/>
    <circle cx={x+245} cy={y+95} r="5" fill="#202020" stroke={colors[m.state]} strokeWidth="2"/>
    <circle cx={endX} cy={endY} r="4" fill="#202020" stroke={m.id===selected?'#fb5102':'#888'}/>
   </g>})}
   {motors.map((m,i)=>{const [x,y]=nodes[i];return <foreignObject key={m.id} x={x} y={y} width="245" height="195">
    <button className={`workflow-card ${m.id===selected?'is-selected':''}`} onClick={()=>onSelect(m.id)} aria-pressed={m.id===selected} aria-label={`${m.id} ${m.name}, ${stateLabels[m.state]}`}>
     <div className="workflow-node-head" onPointerDown={e=>begin(e,i)} title="Arraste para mover o motor"><span className="workflow-motor-icon"><MotorGlyph/></span><span><strong>{m.id}</strong><small>SENSOR · {m.deviceId}</small></span><i className={`dot ${m.state}`}/></div>
     <h3>{m.name}</h3>
     <div className="workflow-node-stats"><div><span>RMS</span><strong>{m.rms?.toFixed(4)??'—'} <small>g</small></strong></div><div><span>Temperatura</span><strong>{m.temperature?.toFixed(1)??'—'} <small>°C</small></strong></div></div>
     <div className="workflow-node-footer"><span className={m.state}>{stateLabels[m.state]}</span><span>{m.rssi} dBm</span></div>
    </button>
   </foreignObject>})}
   <foreignObject x={gateway[0]} y={gateway[1]} width="265" height="240"><div className="workflow-card workflow-gateway"><div className="workflow-node-head" onPointerDown={e=>begin(e,-1)} title="Arraste para mover o gateway"><span className="workflow-gateway-icon"><Icon name="gateways"/></span><span><strong>GW-01</strong><small>ESP32-C3 + RA-02</small></span><i className="dot NORMAL"/></div><h3>Gateway da unidade</h3><div className="workflow-gateway-stats"><div><span>Dispositivos</span><strong>{motors.length}</strong></div><div><span>Fila · demo</span><strong>0 / 64</strong></div><div><span>Transporte</span><strong>LoRa → HTTPS</strong></div></div><div className="workflow-node-footer"><span className="NORMAL">Online · simulado</span><span>DEMO</span></div></div></foreignObject>
   <text x="694" y="790" fill="#777" fontSize="10">Conexões demonstrativas · sem transmissão ativa</text>
  </g>
 </svg>;
}
