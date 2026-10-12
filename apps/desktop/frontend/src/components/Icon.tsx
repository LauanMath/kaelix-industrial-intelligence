import { ChartLineIcon, CpuIcon, BroadcastIcon, ListBulletsIcon, SlidersHorizontalIcon, SidebarSimpleIcon, ExportIcon, MagnifyingGlassIcon, PlusIcon, MinusIcon, CrosshairIcon, InfoIcon, ArrowUpRightIcon } from '@phosphor-icons/react';
const icons = {
 dashboard: monitor: ChartLineIcon, motors: CpuIcon, gateways: BroadcastIcon,
 events: ListBulletsIcon, settings: SlidersHorizontalIcon,
 sidebar: SidebarSimpleIcon, export: ExportIcon, search: MagnifyingGlassIcon,
 plus: PlusIcon, minus: MinusIcon, reset: CrosshairIcon, info: InfoIcon,
 arrow: ArrowUpRightIcon,
} as const;
export function Icon({name}: {name:keyof typeof icons}) {
 const Component=icons[name];
 return <Component className="ui-icon" size={17} weight="regular" aria-hidden="true"/>;
}
