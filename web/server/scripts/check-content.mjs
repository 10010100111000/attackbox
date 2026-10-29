import { checkContentDirectory } from '../content/read-directory.ts'
import { ContentFormatError } from '../content/parse-module.ts'
if (process.argv.length !== 3) {
  console.error('用法：npm --prefix web run content:check -- <教程目录>（路径相对于 web/）')
  process.exitCode = 1
} else {
  try {
    const results = await checkContentDirectory(process.argv[2])
    // Do not print parsed source objects: they contain standard answers.
    console.log(
      JSON.stringify(
        {
          ok: true,
          modules: results.map(({ module, images }) => ({
            file: module.sourcePath,
            title: module.title,
            category: module.categorySlug,
            difficulty: module.difficulty,
            chapterCount: module.chapters.length,
            questionCount: module.chapters.reduce((count, c) => count + c.questions.length, 0),
            imageCount: new Set(images.map(i => i.repositoryPath)).size,
            chapters: module.chapters.map(c => ({
              title: c.title,
              position: c.position,
              access: c.access,
              durationMinutes: c.durationMinutes,
              questionCount: c.questions.length
            }))
          }))
        },
        null,
        2
      )
    )
  } catch (error) {
    console.error(
      JSON.stringify(
        {
          ok: false,
          issues:
            error instanceof ContentFormatError
              ? error.issues
              : [{ file: '.', line: 1, message: '无法读取内容目录，请检查路径和权限' }]
        },
        null,
        2
      )
    )
    process.exitCode = 1
  }
}
