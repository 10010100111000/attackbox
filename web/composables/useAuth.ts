import { clearNuxtData, useNuxtApp } from '#imports';
export interface AuthUser {
  id: string;
  email: string;
  displayName: string;
  role: 'user' | 'admin';
  status: 'pending' | 'active' | 'suspended';
}

export function useAuth() {
  const nuxtApp = useNuxtApp();
  function clearLearningData() { nuxtApp.runWithContext(() => clearNuxtData(key => key.startsWith('learning:'))); }
  const currentUser = useState<AuthUser | null>('auth_user', () => null);
  const isInitialized = useState<boolean>('auth_initialized', () => false);

  async function fetchMe(): Promise<AuthUser | null> {
    try {
      const data = await $fetch<{ user: AuthUser }>('/api/auth/me');
      if (currentUser.value?.id !== data.user.id) clearLearningData();
      currentUser.value = data.user;
      return data.user;
    } catch {
      if (currentUser.value) clearLearningData();
      currentUser.value = null;
      return null;
    } finally {
      isInitialized.value = true;
    }
  }

  async function login(email: string, password: string, rememberMe: boolean = false): Promise<AuthUser> {
    const data = await $fetch<{ user: AuthUser }>('/api/auth/login', {
      method: 'POST',
      body: { email, password, rememberMe },
    });
    clearLearningData();
    currentUser.value = data.user;
    return data.user;
  }

  async function register(email: string, password: string, displayName?: string): Promise<{ message: string; verifyToken?: string }> {
    return await $fetch<{ message: string; verifyToken?: string }>('/api/auth/register', {
      method: 'POST',
      body: { email, password, displayName },
    });
  }

  async function verifyEmail(token: string): Promise<{ message: string }> {
    return await $fetch<{ message: string }>('/api/auth/verify-email', {
      method: 'POST',
      body: { token },
    });
  }

  async function logout(): Promise<void> {
    try {
      await $fetch('/api/auth/logout', { method: 'POST' });
    } finally {
      clearLearningData();
      currentUser.value = null;
    }
  }

  async function forgotPassword(email: string): Promise<{ message: string; resetToken?: string }> {
    return await $fetch<{ message: string; resetToken?: string }>('/api/auth/forgot-password', {
      method: 'POST',
      body: { email },
    });
  }

  async function resetPassword(token: string, newPassword: string): Promise<{ message: string }> {
    return await $fetch<{ message: string }>('/api/auth/reset-password', {
      method: 'POST',
      body: { token, newPassword },
    });
  }

  return {
    currentUser,
    isInitialized,
    fetchMe,
    login,
    register,
    verifyEmail,
    logout,
    forgotPassword,
    resetPassword,
  };
}
