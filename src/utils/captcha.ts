import type { CaptchaPayload } from "@/api/modules/auth";

type GeetestValidateResult = {
  lot_number: string;
  captcha_output: string;
  pass_token: string;
  gen_time: string;
};

type PublicCaptchaConfig = {
  enabled: boolean;
  provider: "geetest" | "cloudflare";
  geetest_captcha_id: string;
  cloudflare_site_key: string;
};

declare global {
  interface Window {
    initGeetest4?: (config: Record<string, unknown>, callback: (captchaObj: any) => void) => void;
    turnstile?: {
      render: (target: string | HTMLElement, options: Record<string, unknown>) => string | number;
      execute: (widgetId: string | number) => void;
      remove: (widgetId: string | number) => void;
    };
  }
}

const scriptLoadingMap = new Map<string, Promise<void>>();

function loadScriptOnce(id: string, src: string) {
  const inFlight = scriptLoadingMap.get(id);
  if (inFlight) return inFlight;

  const existing = document.getElementById(id) as HTMLScriptElement | null;
  if (existing) {
    const done = Promise.resolve();
    scriptLoadingMap.set(id, done);
    return done;
  }

  const task = new Promise<void>((resolve, reject) => {
    const script = document.createElement("script");
    script.id = id;
    script.src = src;
    script.async = true;
    script.defer = true;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("Failed to load captcha script"));
    document.head.appendChild(script);
  });

  scriptLoadingMap.set(id, task);
  return task;
}

async function ensureGeetestScript() {
  await loadScriptOnce("captcha-geetest-v4", "https://static.geetest.com/v4/gt4.js");
}

async function ensureCloudflareScript() {
  await loadScriptOnce("captcha-cloudflare-turnstile", "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit");
}

function shouldRunCaptcha(config: PublicCaptchaConfig | undefined, scene: "login" | "register") {
  if (!config?.enabled) return false;
  return scene === "login" || scene === "register";
}

export async function resolveCaptchaPayload(
  config: PublicCaptchaConfig | undefined,
  scene: "login" | "register"
): Promise<CaptchaPayload | undefined> {
  if (!shouldRunCaptcha(config, scene) || !config) {
    return undefined;
  }
  const provider = config.provider || "geetest";
  if (provider === "cloudflare") {
    const token = await runCloudflareTurnstile(config.cloudflare_site_key);
    return {
      provider: "cloudflare",
      cloudflare_token: token
    };
  }

  const result = await runGeetestBind(config.geetest_captcha_id);
  return {
    provider: "geetest",
    geetest_lot_number: result.lot_number,
    geetest_captcha_output: result.captcha_output,
    geetest_pass_token: result.pass_token,
    geetest_gen_time: result.gen_time
  };
}

async function runGeetestBind(captchaID: string): Promise<GeetestValidateResult> {
  const normalizedCaptchaID = (captchaID || "").trim();
  if (!normalizedCaptchaID) {
    throw new Error("Geetest captcha id is not configured");
  }

  await ensureGeetestScript();
  if (typeof window.initGeetest4 !== "function") {
    throw new Error("Failed to initialize Geetest script");
  }

  return new Promise<GeetestValidateResult>((resolve, reject) => {
    let settled = false;
    let timer: number | null = null;
    let captchaInstance: any = null;

    const done = (handler: () => void) => {
      if (settled) return;
      settled = true;
      if (timer) {
        window.clearTimeout(timer);
      }
      try {
        captchaInstance?.destroy?.();
      } catch {
        // ignore cleanup failure
      }
      handler();
    };

    timer = window.setTimeout(() => {
      done(() => reject(new Error("Captcha timeout, please try again")));
    }, 120000);

    try {
      window.initGeetest4?.(
        {
          captchaId: normalizedCaptchaID,
          product: "bind"
        },
        (captchaObj) => {
          captchaInstance = captchaObj;
          captchaObj.onSuccess?.(() => {
            const result = captchaObj.getValidate?.() as GeetestValidateResult | null;
            if (
              result &&
              result.lot_number &&
              result.captcha_output &&
              result.pass_token &&
              result.gen_time
            ) {
              done(() => resolve(result));
              return;
            }
            done(() => reject(new Error("Geetest returned invalid payload")));
          });
          captchaObj.onError?.(() => {
            done(() => reject(new Error("Geetest verification failed")));
          });
          captchaObj.onClose?.(() => {
            done(() => reject(new Error("Captcha verification canceled")));
          });

          if (typeof captchaObj.showCaptcha === "function") {
            captchaObj.showCaptcha();
          } else {
            done(() => reject(new Error("Geetest instance is unavailable")));
          }
        }
      );
    } catch {
      done(() => reject(new Error("Failed to start Geetest verification")));
    }
  });
}

async function runCloudflareTurnstile(siteKey: string): Promise<string> {
  const normalizedSiteKey = (siteKey || "").trim();
  if (!normalizedSiteKey) {
    throw new Error("Cloudflare site key is not configured");
  }

  await ensureCloudflareScript();
  const turnstile = window.turnstile;
  if (!turnstile) {
    throw new Error("Failed to initialize Cloudflare script");
  }

  return new Promise<string>((resolve, reject) => {
    let settled = false;
    let timer: number | null = null;
    const container = document.createElement("div");
    container.style.position = "fixed";
    container.style.right = "-9999px";
    container.style.bottom = "-9999px";
    document.body.appendChild(container);

    let widgetID: string | number | null = null;
    const done = (handler: () => void) => {
      if (settled) return;
      settled = true;
      if (timer) {
        window.clearTimeout(timer);
      }
      try {
        if (widgetID !== null) {
          turnstile.remove(widgetID);
        }
      } catch {
        // ignore cleanup failure
      }
      container.remove();
      handler();
    };

    timer = window.setTimeout(() => {
      done(() => reject(new Error("Captcha timeout, please try again")));
    }, 120000);

    try {
      widgetID = turnstile.render(container, {
        sitekey: normalizedSiteKey,
        execution: "execute",
        appearance: "interaction-only",
        callback: (token: string) => {
          const text = (token || "").trim();
          if (!text) {
            done(() => reject(new Error("Cloudflare did not return a valid token")));
            return;
          }
          done(() => resolve(text));
        },
        "error-callback": () => {
          done(() => reject(new Error("Cloudflare verification failed")));
        },
        "expired-callback": () => {
          done(() => reject(new Error("Cloudflare token expired")));
        },
        "timeout-callback": () => {
          done(() => reject(new Error("Cloudflare verification timed out")));
        }
      });
      turnstile.execute(widgetID);
    } catch {
      done(() => reject(new Error("Failed to start Cloudflare verification")));
    }
  });
}
