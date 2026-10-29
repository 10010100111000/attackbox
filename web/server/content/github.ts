import { createError } from 'h3'

export interface RepositoryConfig {
  repository: string
  branch: string
  directory: string
}

export interface GithubTreeEntry {
  path: string
  mode: string
  type: 'blob' | 'tree' | string
  sha: string
  size?: number
}

export function parseRepositoryConfig(body: Record<string, unknown>): RepositoryConfig {
  if (
    typeof body.repository !== 'string' ||
    (body.branch !== undefined && typeof body.branch !== 'string') ||
    (body.directory !== undefined && typeof body.directory !== 'string')
  ) {
    throw createError({ statusCode: 400, message: '仓库、分支和目录必须是文本' })
  }
  const repository = String(body.repository || '')
    .trim()
    .replace(/^https:\/\/github\.com\//, '')
    .replace(/\.git\/?$/, '')
    .replace(/\/$/, '')
  if (
    !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository) ||
    repository.split('/').some(p => p === '.' || p === '..')
  ) {
    throw createError({
      statusCode: 400,
      message: '请填写 GitHub owner/repository 或标准 HTTPS 仓库链接'
    })
  }
  const branch = String(body.branch || 'main').trim()
  const directory = String(body.directory || '')
    .trim()
    .replace(/^\.\/$/, '')
    .replace(/\/$/, '')
  if (!branch || branch.length > 200 || /[\s\\?#]/.test(branch)) {
    throw createError({ statusCode: 400, message: '分支名无效' })
  }
  if (directory.length > 500 || directory.split('/').length > 16) {
    throw createError({ statusCode: 400, message: '教程目录过长或层级过深' })
  }
  if (
    directory &&
    (directory.startsWith('/') ||
      directory.includes('\\') ||
      directory.split('/').some(p => !p || p === '.' || p === '..'))
  ) {
    throw createError({ statusCode: 400, message: '教程目录必须是仓库内相对路径' })
  }
  return { repository, branch, directory }
}

// Fixed GitHub API host; no arbitrary URLs, redirects, shell commands or repo scripts.
export async function githubJson(
  config: RepositoryConfig,
  token: string,
  suffix: string,
  signal?: AbortSignal
): Promise<any> {
  try {
    return await $fetch(`https://api.github.com/repos/${config.repository}/${suffix}`, {
      headers: {
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        Accept: 'application/vnd.github+json',
        'X-GitHub-Api-Version': '2022-11-28'
      },
      timeout: 20000,
      retry: 0,
      redirect: 'error',
      signal
    })
  } catch (e: any) {
    const status = e?.response?.status
    throw createError({
      statusCode: 502,
      message:
        status === 401 || status === 403
          ? 'GitHub 凭据无效、权限不足或访问受限'
          : status === 404
            ? '仓库、分支或目录不存在，或 Token 没有读取权限'
            : 'GitHub 访问失败，请稍后手动重试'
    })
  }
}

export async function githubCommit(config: RepositoryConfig, token: string, signal?: AbortSignal) {
  const result = await githubJson(
    config,
    token,
    `commits/${encodeURIComponent(config.branch)}`,
    signal
  )
  if (!/^[a-f0-9]{40}$/.test(result.sha)) {
    throw createError({ statusCode: 502, message: 'GitHub 返回无效提交' })
  }
  return result.sha as string
}

export async function githubDirectory(
  config: RepositoryConfig,
  token: string,
  commit: string,
  signal?: AbortSignal
) {
  let tree = (await githubJson(config, token, `git/commits/${commit}`, signal)).tree?.sha
  for (const component of config.directory.split('/').filter(Boolean)) {
    const result = await githubJson(config, token, `git/trees/${tree}`, signal)
    const entry = result.tree?.find((e: any) => e.path === component && e.type === 'tree')
    if (!entry) throw createError({ statusCode: 400, message: '指定教程目录不存在' })
    tree = entry.sha
  }
  return tree as string
}

/** Single lightweight metadata request returning all paths and Git blob SHAs without downloading file bytes. */
export async function fetchGithubTree(config: RepositoryConfig, token: string, subFolder?: string) {
  const signal = AbortSignal.timeout(30000)
  const targetConfig: RepositoryConfig = subFolder
    ? { ...config, directory: [config.directory, subFolder].filter(Boolean).join('/') }
    : config
  const commit = await githubCommit(targetConfig, token, signal)
  const treeSha = await githubDirectory(targetConfig, token, commit, signal)
  const result = await githubJson(targetConfig, token, `git/trees/${treeSha}?recursive=1`, signal)
  if (result.truncated || !Array.isArray(result.tree) || result.tree.length > 5000) {
    throw createError({ statusCode: 400, message: '内容目录文件数量过多，请指定更具体的子目录' })
  }
  for (const entry of result.tree as GithubTreeEntry[]) {
    if (entry.mode === '120000' || (entry.type !== 'blob' && entry.type !== 'tree')) {
      throw createError({
        statusCode: 400,
        message: `教程目录不支持符号链接或子模块 (${entry.path})`
      })
    }
  }
  return { commit, treeSha, entries: result.tree as GithubTreeEntry[] }
}

/** Downloads a single file blob by its Git SHA only when its content is needed. */
export async function fetchGithubBlob(
  config: RepositoryConfig,
  token: string,
  sha: string,
  expectedSize?: number
): Promise<Buffer> {
  const signal = AbortSignal.timeout(60000)
  const blob = await githubJson(config, token, `git/blobs/${sha}`, signal)
  if (blob.encoding !== 'base64' || typeof blob.content !== 'string') {
    throw createError({ statusCode: 502, message: 'GitHub 文件编码无效' })
  }
  const data = Buffer.from(blob.content, 'base64')
  if (
    (expectedSize !== undefined && data.length !== expectedSize) ||
    data.length > 10 * 1024 * 1024
  ) {
    throw createError({ statusCode: 502, message: 'GitHub 文件大小不一致或超出 10 MiB 限制' })
  }
  return data
}
