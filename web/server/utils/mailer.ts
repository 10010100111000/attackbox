export interface MailerService {
  sendVerificationEmail(to: string, token: string): Promise<void>;
  sendPasswordResetEmail(to: string, token: string): Promise<void>;
}

export class ConsoleMailerService implements MailerService {
  async sendVerificationEmail(to: string, token: string): Promise<void> {
    const verifyUrl = `/auth?mode=verify&token=${token}`;
    console.log('--------------------------------------------------');
    console.log(`[MAILER] Verification email sent to: ${to}`);
    console.log(`[MAILER] Activation Link: ${verifyUrl}`);
    console.log(`[MAILER] Token: ${token}`);
    console.log('--------------------------------------------------');
  }

  async sendPasswordResetEmail(to: string, token: string): Promise<void> {
    const resetUrl = `/auth?mode=reset&token=${token}`;
    console.log('--------------------------------------------------');
    console.log(`[MAILER] Password reset email sent to: ${to}`);
    console.log(`[MAILER] Reset Link: ${resetUrl}`);
    console.log(`[MAILER] Token: ${token}`);
    console.log('--------------------------------------------------');
  }
}

export const mailer = new ConsoleMailerService();
