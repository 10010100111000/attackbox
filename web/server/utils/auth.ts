import { hash, verify } from '@node-rs/argon2';
import crypto from 'node:crypto';
import { type H3Event, getCookie, setCookie, deleteCookie, getRequestIP, getRequestHeader } from 'h3';

export const SESSION_COOKIE_NAME = 'attackbox_session';

/**
 * 使用 Argon2id 计算密码安全哈希
 */
export async function hashPassword(password: string): Promise<string> {
  return hash(password, {
    algorithm: 2, // Argon2id; const enum cannot be imported under verbatimModuleSyntax.
    memoryCost: 65536, // 64 MB
    timeCost: 3,
    parallelism: 1,
  });
}

/**
 * 校验明文密码与 Argon2id 哈希
 */
export async function verifyPassword(hashStr: string, plain: string): Promise<boolean> {
  try {
    return await verify(hashStr, plain);
  } catch {
    return false;
  }
}

/**
 * 生成 32 字节高熵随机串（64 字符十六进制）
 */
export function generateSecureToken(): string {
  return crypto.randomBytes(32).toString('hex');
}

/**
 * 将原始 Token 转换为 32 字节 SHA-256 Buffer (存储于 PostgreSQL bytea)
 */
export function hashToken(token: string): Buffer {
  return crypto.createHash('sha256').update(token).digest();
}

/**
 * 设置会话 HttpOnly Cookie
 */
export function setSessionCookie(event: H3Event, token: string, rememberMe: boolean = false): void {
  // 常规 24 小时，记住我 30 天
  const maxAge = rememberMe ? 86400 * 30 : 86400;
  const socket = event.node?.req?.socket;
  const encrypted = socket && 'encrypted' in socket && socket.encrypted === true;
  const proto = getRequestHeader(event, 'x-forwarded-proto') || (encrypted ? 'https' : 'http');
  const isSecure = proto === 'https';

  setCookie(event, SESSION_COOKIE_NAME, token, {
    httpOnly: true,
    secure: isSecure,
    sameSite: 'lax',
    path: '/',
    maxAge,
  });
}

/**
 * 清除会话 Cookie
 */
export function clearSessionCookie(event: H3Event): void {
  deleteCookie(event, SESSION_COOKIE_NAME, {
    path: '/',
  });
}

/**
 * 获取请求携带的会话 Token
 */
export function getSessionToken(event: H3Event): string | undefined {
  return getCookie(event, SESSION_COOKIE_NAME);
}

/**
 * 获取客户端 IP 地址
 */
export function getEventIp(event: H3Event): string | undefined {
  return getRequestIP(event, { xForwardedFor: true }) || undefined;
}
