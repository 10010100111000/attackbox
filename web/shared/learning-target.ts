export type LearningTarget = null | {status:'unavailable'} | {status:'configured';scope:'module'|'chapter';name:string;description:string;entryPort:number;accessInstructions:string}
