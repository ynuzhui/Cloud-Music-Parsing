import http from "../http";

export type UserQuotaToday = {
  timezone: string;
  date: string;
  daily_limit: number;
  used: number;
  remaining: number;
  concurrency_limit: number;
  in_flight: number;
};

export type UserUsageTrend = {
  timezone: string;
  days: number;
  items: Array<{ day: string; count: number }>;
};

export function getUserQuotaToday() {
  return http.get<never, UserQuotaToday>("/api/user/quota/today");
}

export function getUserUsageTrend(days = 7) {
  return http.get<never, UserUsageTrend>("/api/user/usage/trend", { params: { days } });
}

