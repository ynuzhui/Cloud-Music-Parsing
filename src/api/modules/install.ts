import http from "../http";

export type InstallDBConfig = {
  driver: "sqlite" | "mysql";
  sqlite_path: string;
  mysql_host: string;
  mysql_port: string;
  mysql_user: string;
  mysql_pass: string;
  mysql_db: string;
  mysql_param: string;
};

export type HealthStatus = {
  status: string;
  installed: boolean;
};

export type InstallCompletePayload = {
  database: InstallDBConfig;
  admin_username: string;
  admin_email: string;
  admin_password: string;
  site_name: string;
};

export function testDatabase(database: InstallDBConfig) {
  return http.post("/api/install/test-db", { database });
}

export function completeInstall(payload: InstallCompletePayload) {
  return http.post("/api/install/complete", payload);
}

export function getHealthStatus() {
  return http.get<never, HealthStatus>("/api/health");
}
