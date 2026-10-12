export function Brand({compact=false}: {compact?:boolean}) {
 return <span className="brand-lockup"><img className="brand-symbol" src="/brand/kaelix-mark.svg" alt=""/>{!compact&&<span className="brand-type">KAELIX<small>INDUSTRIAL INTELLIGENCE</small></span>}</span>;
}
