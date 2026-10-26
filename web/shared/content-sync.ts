export interface ContentSyncBindings {modules:Record<string,string>;chapters:Record<string,string>;questions:Record<string,string>}
export interface ContentPreviewReport {
 id:string;applied:boolean;result?:{moduleIds:string[]};commit?:string|null;expiresAt?:string;stale?:boolean;bindings?:ContentSyncBindings;conflicts?:string[];removals?:string[];missingModules?:{id:string;path:string;title:string}[]
 candidates?:{id:string;title:string;chapters:{id:string;title:string;questions:{id:string;prompt:string}[]}[]}[]
 modules?:{path:string;id:string|null;changed:boolean;preview:{imageUrls?:Record<string,string>;title:string;summary:string;descriptionMarkdown:string;learningOutcomes:string[];chapters:{title:string;bodyMarkdown:string;access:string;durationMinutes:number|null;questions:{promptMarkdown:string}[]}[]};chapters:{key:string;id:string|null;title:string;questions:{key:string;id:string|null;prompt:string}[]}[]}[]
}
