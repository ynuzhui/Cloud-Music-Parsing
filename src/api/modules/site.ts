import http from "../http";

export type PublicSiteSettings = {
  name: string;
  keywords: string;
  description: string;
  icp_no: string;
  police_no: string;
  allow_register: boolean;
  register_email_verify: boolean;
  parse_require_login: boolean;
  captcha?: {
    enabled: boolean;
    provider: "geetest" | "cloudflare";
    geetest_captcha_id: string;
    cloudflare_site_key: string;
  };
  timezone: string;
};

let siteSettingsCache: PublicSiteSettings | null = null;
let siteSettingsInFlight: Promise<PublicSiteSettings> | null = null;

export function getPublicSiteSettings(force = false) {
  if (!force) {
    if (siteSettingsCache) return Promise.resolve(siteSettingsCache);
    if (siteSettingsInFlight) return siteSettingsInFlight;
  }

  siteSettingsInFlight = http
    .get<never, PublicSiteSettings>("/api/site")
    .then((site) => {
      siteSettingsCache = site;
      return site;
    })
    .finally(() => {
      siteSettingsInFlight = null;
    });

  return siteSettingsInFlight;
}

export function clearPublicSiteSettingsCache() {
  siteSettingsCache = null;
  siteSettingsInFlight = null;
}
