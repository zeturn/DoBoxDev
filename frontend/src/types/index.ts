export interface User {
  id: number;
  username: string;
  email: string;
  created_at: string;
  updated_at: string;
}

export interface Container {
  id: number;
  user_id: number;
  container_id: string;
  name: string;
  image: string;
  status: string;
  ports: string;
  env_vars: string;
  volumes: string;
  command: string;
  working_dir: string;
  restart_policy: string;
  network_mode: string;
  cpu_limit: number;
  memory_limit: number;
  created_at: string;
  updated_at: string;
}

export interface ContainerStats {
  cpu_percent: number;
  memory_usage: number;
  memory_limit: number;
  memory_percent: number;
  network_rx: number;
  network_tx: number;
  disk_read: number;
  disk_write: number;
}

export interface CreateContainerRequest {
  name: string;
  image: string;
  env?: string[];
  ports?: Record<string, string>;
  volumes?: string[];
  command?: string[];
  working_dir?: string;
  restart_policy?: string;
  network_mode?: string;
  cpu_limit?: number;
  memory_limit?: number;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
}

export interface ImageSummary {
  id: string;
  repo_tags: string[];
  created: number;
  size: number;
}

export interface OperationAudit {
  id: number;
  user_id: number;
  container_id: number;
  action: string;
  status: string;
  detail: string;
  created_at: string;
}

export interface InfraOperationAudit extends OperationAudit {}

export interface NetworkSummary {
  id: string;
  name: string;
  driver: string;
  scope: string;
  containers: number;
}

export interface VolumeSummary {
  name: string;
  driver: string;
  scope: string;
  mountpoint: string;
  created_at: string;
  labels?: Record<string, string>;
}
