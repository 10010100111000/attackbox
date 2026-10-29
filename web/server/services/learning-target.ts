import { eq } from 'drizzle-orm'
import { chapters, modules, targetTemplates } from '../db/schema'
import type { LearningTx } from './learning'
import type { LearningTarget } from '../../shared/learning-target'
// Call only after chapterContext has enforced publication and chapter access.
export async function learningTarget(tx: LearningTx, chapterId: string): Promise<LearningTarget> {
  const [config] = await tx
    .select({
      mode: chapters.targetMode,
      chapterTemplateId: chapters.targetTemplateId,
      moduleTemplateId: modules.defaultTargetTemplateId
    })
    .from(chapters)
    .innerJoin(modules, eq(chapters.moduleId, modules.id))
    .where(eq(chapters.id, chapterId))
  if (!config || config.mode === 'none') return null
  const id = config.mode === 'custom' ? config.chapterTemplateId : config.moduleTemplateId
  if (!id) return null
  const [template] = await tx
    .select({
      name: targetTemplates.name,
      description: targetTemplates.description,
      settings: targetTemplates.settings,
      accessInstructions: targetTemplates.accessInstructions,
      enabled: targetTemplates.enabled
    })
    .from(targetTemplates)
    .where(eq(targetTemplates.id, id))
  if (!template?.enabled) return { status: 'unavailable' }
  return {
    status: 'configured',
    scope: config.mode === 'custom' ? 'chapter' : 'module',
    name: template.name,
    description: template.description,
    entryPort: template.settings.entryPort,
    accessInstructions: template.accessInstructions
  }
}
