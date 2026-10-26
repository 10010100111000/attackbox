export interface TargetPort { port:number; protocol:'tcp'|'udp' }
export interface TargetTemplateInput { name:string; description:string; image:string; sourceUrl:string; ports:TargetPort[]; accessInstructions:string; enabled:boolean }
export interface TargetTemplate extends TargetTemplateInput { id:string; lockVersion:number; createdAt:string; updatedAt:string;runtime?:'container'|'compose';bundleId?:string|null;composeRevision?:number;composeSettings?:import('./compose-labs').ComposeSettings|null;variableKeys?:string[] }
export interface TargetTemplateList { items:TargetTemplate[]; total:number; page:number; pageSize:number }
