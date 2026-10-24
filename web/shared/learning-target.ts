import type {TargetPort} from './target-templates'
export type LearningTarget = null | {status:'unavailable'} | {status:'configured';scope:'module'|'chapter';name:string;description:string;ports:TargetPort[];accessInstructions:string}
