import axios from 'axios';
import type { AxiosInstance } from 'axios';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:3000/api';

class ApiService {
  private api: AxiosInstance;

  constructor() {
    this.api = axios.create({
      baseURL: API_BASE_URL,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Add token to requests if available
    this.api.interceptors.request.use((config) => {
      const token = localStorage.getItem('token');
      if (token) {
        config.headers.Authorization = `Bearer ${token}`;
      }
      return config;
    });

    // Handle 401 responses
    this.api.interceptors.response.use(
      (response) => response,
      (error) => {
        const status = error.response?.status;
        const requestUrl = String(error.config?.url || '');
        const isAuthRequest =
          requestUrl.includes('/auth/login') || requestUrl.includes('/auth/register');

        if (status === 401 && !isAuthRequest) {
          localStorage.removeItem('token');
          localStorage.removeItem('user');
          if (window.location.pathname !== '/login') {
            window.location.href = '/login';
          }
        }
        return Promise.reject(error);
      }
    );
  }

  // Auth
  async register(username: string, email: string, password: string) {
    const response = await this.api.post('/auth/register', { username, email, password });
    return response.data;
  }

  async login(username: string, password: string) {
    const response = await this.api.post('/auth/login', { username, password });
    return response.data;
  }

  async getMe() {
    const response = await this.api.get('/auth/me');
    return response.data;
  }

  // Containers
  async listContainers() {
    const response = await this.api.get('/containers');
    return response.data;
  }

  async createContainer(data: any) {
    const response = await this.api.post('/containers', data);
    return response.data;
  }

  async getContainer(id: number) {
    const response = await this.api.get(`/containers/${id}`);
    return response.data;
  }

  async startContainer(id: number) {
    const response = await this.api.post(`/containers/${id}/start`);
    return response.data;
  }

  async stopContainer(id: number) {
    const response = await this.api.post(`/containers/${id}/stop`);
    return response.data;
  }

  async restartContainer(id: number) {
    const response = await this.api.post(`/containers/${id}/restart`);
    return response.data;
  }

  async pauseContainer(id: number) {
    const response = await this.api.post(`/containers/${id}/pause`);
    return response.data;
  }

  async unpauseContainer(id: number) {
    const response = await this.api.post(`/containers/${id}/unpause`);
    return response.data;
  }

  async deleteContainer(id: number) {
    const response = await this.api.delete(`/containers/${id}`);
    return response.data;
  }

  async updateLimits(id: number, cpuLimit: number, memoryLimit: number) {
    const response = await this.api.put(`/containers/${id}/limits`, {
      cpu_limit: cpuLimit,
      memory_limit: memoryLimit,
    });
    return response.data;
  }

  async getContainerLogs(
    id: number,
    options?: { tail?: number; since?: string; until?: string; follow?: boolean }
  ) {
    const params = new URLSearchParams();
    params.set('tail', String(options?.tail ?? 100));
    if (options?.since) params.set('since', options.since);
    if (options?.until) params.set('until', options.until);
    if (options?.follow) params.set('follow', 'true');
    const response = await this.api.get(`/containers/${id}/logs?${params.toString()}`);
    return response.data;
  }

  async getContainerStats(id: number) {
    const response = await this.api.get(`/containers/${id}/stats`);
    return response.data;
  }

  async execInContainer(id: number, payload: { command: string[]; working_dir?: string; env?: string[] }) {
    const response = await this.api.post(`/containers/${id}/exec`, payload);
    return response.data;
  }

  async getContainerProcesses(id: number) {
    const response = await this.api.get(`/containers/${id}/processes`);
    return response.data;
  }

  async getContainerState(id: number) {
    const response = await this.api.get(`/containers/${id}/state`);
    return response.data;
  }

  async uploadContainerFile(id: number, payload: { destination_path: string; file_name: string; content_base64: string }) {
    const response = await this.api.post(`/containers/${id}/files/upload`, payload);
    return response.data;
  }

  async downloadContainerFile(id: number, path: string) {
    const response = await this.api.get(`/containers/${id}/files/download?path=${encodeURIComponent(path)}`);
    return response.data;
  }

  async listContainerAudits(id: number) {
    const response = await this.api.get(`/containers/${id}/audits`);
    return response.data;
  }

  async listInfraAudits() {
    const response = await this.api.get('/containers/audits/all?scope=infra');
    return response.data;
  }

  // Networks
  async createNetwork(payload: { name: string; driver?: string; attachable?: boolean; internal?: boolean }) {
    const response = await this.api.post('/networks', payload);
    return response.data;
  }

  async listNetworks() {
    const response = await this.api.get('/networks');
    return response.data;
  }

  async inspectNetwork(id: string) {
    const response = await this.api.get(`/networks/${encodeURIComponent(id)}`);
    return response.data;
  }

  async deleteNetwork(id: string) {
    const response = await this.api.delete(`/networks/${encodeURIComponent(id)}`);
    return response.data;
  }

  async connectContainerToNetwork(networkId: string, containerId: number) {
    const response = await this.api.post(`/networks/${encodeURIComponent(networkId)}/connect`, { container_id: String(containerId) });
    return response.data;
  }

  async disconnectContainerFromNetwork(networkId: string, containerId: number, force = false) {
    const response = await this.api.post(`/networks/${encodeURIComponent(networkId)}/disconnect`, { container_id: String(containerId), force });
    return response.data;
  }

  // Volumes
  async createVolume(payload: { name: string; driver?: string; labels?: Record<string, string> }) {
    const response = await this.api.post('/volumes', payload);
    return response.data;
  }

  async listVolumes() {
    const response = await this.api.get('/volumes');
    return response.data;
  }

  async inspectVolume(name: string) {
    const response = await this.api.get(`/volumes/${encodeURIComponent(name)}`);
    return response.data;
  }

  async deleteVolume(name: string, force = false) {
    const response = await this.api.delete(`/volumes/${encodeURIComponent(name)}?force=${force}`);
    return response.data;
  }

  async getVolumeRelations(name: string) {
    const response = await this.api.get(`/volumes/${encodeURIComponent(name)}/relations`);
    return response.data;
  }

  // Images
  async pullImage(image: string) {
    const response = await this.api.post('/images/pull', { image });
    return response.data;
  }

  async listImages() {
    const response = await this.api.get('/images');
    return response.data;
  }

  async inspectImage(ref: string) {
    const response = await this.api.get(`/images/${encodeURIComponent(ref)}`);
    return response.data;
  }

  async deleteImage(ref: string, force = false) {
    const response = await this.api.delete(`/images/${encodeURIComponent(ref)}?force=${force}`);
    return response.data;
  }

  async tagImage(source: string, target: string) {
    const response = await this.api.post('/images/tag', { source, target });
    return response.data;
  }

  async pushImage(image: string, username: string, password: string, server_address: string) {
    const response = await this.api.post('/images/push', { image, username, password, server_address });
    return response.data;
  }

  async buildImage(payload: {
    context_path: string;
    dockerfile: string;
    tag: string;
    build_args?: Record<string, string>;
    no_cache?: boolean;
  }) {
    const response = await this.api.post('/images/build', payload);
    return response.data;
  }
}

export default new ApiService();
