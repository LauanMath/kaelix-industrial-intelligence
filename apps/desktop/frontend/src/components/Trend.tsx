export function Trend({values, large=false}: {values:(number|null)[]; large?:boolean}) {
 const w=600,h=large?170:50,max=Math.max(...values.filter((v):v is number=>v!==null),.001)*1.25;
 const segments:string[]=[]; let segment=''; values.forEach((v,i)=>{ if(v===null){ if(segment)segments.push(segment);segment=''; }else{segment+=`${segment?' L':'M'}${i*w/(values.length-1)},${h-v/max*(h-10)}`;} });if(segment)segments.push(segment);
 return <svg className="trend" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" role="img" aria-label="Histórico de RMS em g">
 {large&&[.25,.5,.75,1].map(n=><line key={n} x1="0" x2={w} y1={n*h} y2={n*h} stroke="#ffffff0a"/>)}
 {segments.map((d,i)=><path key={i} d={d} fill="none" stroke="currentColor" strokeWidth={large?2:1.8} vectorEffect="non-scaling-stroke"/>)}
 </svg>
}
