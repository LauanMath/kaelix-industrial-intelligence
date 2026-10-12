export function Trend({values,timestamps,large=false,expectedInterval=0}:{values:(number|null)[];timestamps?: (string|null)[];large?:boolean;expectedInterval?:number}){
 const w=600,h=large?170:50,max=Math.max(...values.filter((v):v is number=>v!==null),.001)*1.25;
 const times=timestamps?.map(t=>t?new Date(t).getTime():NaN),validTimes=times?.filter(Number.isFinite)??[],start=Math.min(...validTimes),end=Math.max(...validTimes);
 const x=(i:number)=>times&&Number.isFinite(times[i])&&end>start?(times[i]-start)/(end-start)*w:values.length>1?i*w/(values.length-1):w/2;
 const y=(value:number)=>h-value/max*(h-10);
 const segments:string[]=[];let segment='';
 values.forEach((value,i)=>{if(value===null){if(segment)segments.push(segment);segment='';return}if(times&&i>0&&expectedInterval>0&&times[i]-times[i-1]>expectedInterval*3000){if(segment)segments.push(segment);segment=''}segment+=`${segment?' L':'M'}${x(i)},${y(value)}`});if(segment)segments.push(segment);
 return <svg className="trend" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" role="img" aria-label={values.length?'Histórico recebido de RMS em g':'Sem leituras no período'}>{large&&[.25,.5,.75,1].map(n=><line key={n} x1="0" x2={w} y1={n*h} y2={n*h} stroke="#ffffff0a"/>)}{segments.map((d,i)=><path key={i} d={d} fill="none" stroke="currentColor" strokeWidth={large?2:1.8} vectorEffect="non-scaling-stroke"/>)}{values.length===1&&values[0]!==null&&<circle cx={w/2} cy={y(values[0])} r="3" fill="currentColor"/>}</svg>
}
