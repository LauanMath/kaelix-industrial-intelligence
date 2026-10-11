import {Motor, stateLabels} from '../lib/demo';
export function Factory({motors, selected, onSelect}: {motors:Motor[];selected:string;onSelect:(id:string)=>void}) {
 const positions=[[120,160],[365,150],[610,140],[130,390],[375,380],[620,370]];
 return <svg className="factory" viewBox="0 0 960 690" role="group" aria-label="Planta industrial demonstrativa. Selecione um motor.">
 <defs><pattern id="grid" width="42" height="24" patternUnits="userSpaceOnUse"><path d="M0 24L42 0M0 0L42 24" stroke="#ffffff" strokeOpacity=".035" fill="none"/></pattern><linearGradient id="metal" x2="0" y2="1"><stop stopColor="#6b7374"/><stop offset="1" stopColor="#252b2d"/></linearGradient><linearGradient id="floor" x2="0" y2="1"><stop stopColor="#33393b"/><stop offset="1" stopColor="#191e20"/></linearGradient><filter id="glow"><feGaussianBlur stdDeviation="4"/></filter></defs>
 <rect width="960" height="690" fill="url(#grid)"/>
 <path d="M45 325L480 105L918 322L485 561Z" fill="url(#floor)" stroke="#4b5455"/><path d="M45 325V345L485 582L918 343V322L485 561Z" fill="#121719" stroke="#394143"/>
 <g fill="none" stroke="#647071" strokeWidth="2" opacity=".4"><path d="M88 343L480 145L875 341M295 220L700 427M170 285L573 493"/><path d="M87 368L477 565M87 359L477 556"/></g>
 {motors.map((m,i)=>{const [x,y]=positions[i],color=m.state==='NORMAL'?'#8ee5a3':m.state==='ATTENTION'?'#edb66c':'#89959e',active=m.id===selected;return <g key={m.id} className="machine" transform={`translate(${x} ${y})`} onClick={()=>onSelect(m.id)} onKeyDown={e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();onSelect(m.id)}}} tabIndex={0} role="button" aria-label={`${m.id} ${m.name}, ${stateLabels[m.state]}`} aria-pressed={active}>
 <ellipse cx="105" cy="132" rx="115" ry="35" fill="#000" opacity=".3"/>
 <path d="M0 95L95 47L210 105L115 153Z" fill="#202729" stroke={active?color:'#576364'} strokeWidth={active?2:1}/><path d="M0 95V108L115 166L210 118V105L115 153Z" fill="#121719" stroke="#3a4548"/>
 {active&&<path d="M0 95L95 47L210 105L115 153Z" fill={color} opacity=".15" filter="url(#glow)"/>}
 <path d="M28 87L97 52L169 88L100 123Z" fill="#697171"/><path d="M28 87V102L100 140V123Z" fill="#353d3f"/><path d="M100 123L169 88V103L100 140Z" fill="#242d30"/>
 <path d="M65 78V36L103 17L140 36V79L103 98Z" fill="url(#metal)" stroke="#7c8888"/><path d="M65 36L103 56L140 36M103 56V98" fill="none" stroke="#a3aeaa" strokeOpacity=".5"/>
 {[0,1,2,3].map(n=><path key={n} d={`M69 ${47+n*8}L98 ${62+n*8}`} stroke="#172022" strokeWidth="3"/>)}
 <path d="M140 62L167 49L182 58V86L165 96L140 83Z" fill="#404c4b" stroke="#75827b"/><circle cx="168" cy="72" r="10" fill="#1b2728" stroke={color}/><circle cx="168" cy="72" r="4" fill={color}/>
 <path d="M13 85V15L106 -32L200 15V89M13 15L106 62L200 15M106 62V135" fill="none" stroke={active?color:'#687574'} strokeOpacity={active?.65:.25}/>
 <rect x="59" y="-58" width="114" height="27" rx="5" fill="#141b1d" stroke={active?color:'#445053'}/><circle cx="73" cy="-44" r="3" fill={color}/><text x="85" y="-40" fill="#e0e6e2" fontSize="12">{m.id}<tspan fill={color} dx="9" fontSize="10">{m.rms===null?'—':m.rms.toFixed(4)}</tspan></text>
 </g>})}
 <g transform="translate(727 520)"><path d="M0 0L45 -23L77 -6V37L33 60L0 43Z" fill="#293337" stroke="#667475"/><path d="M0 0L33 17L77 -6M33 17V60" stroke="#667475" fill="none"/><circle cx="16" cy="24" r="3" fill="#8ee5a3"/><text x="-5" y="83" fill="#8e9a9e" fontSize="11">GW-01 · LoRa</text><path d="M45 -23V-62M35 -53Q45 -67 55 -53M28 -61Q45 -83 62 -61" stroke="#8ee5a3" fill="none" strokeWidth="2"/></g>
 </svg>
}
