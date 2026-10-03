export interface BuildConfiguration {
  executable: string;
  args: string[];
  working_dir: string;
  scanner_policy_version?: string;
}

export interface CommandScanResult {
  status: string;
  policy_version: string;
  source: string;
  detected_framework?: string;
  message?: string;
  denied_reason?: string;
}

export interface Project {
  id: string;
  user_id: string;
  installation_id: string;
  github_repository_id: number;
  repository_owner: string;
  repository_name: string;
  repository_full_name: string;
  configured_branch: string;
  project_webhook_enabled: boolean;
  repository_available: boolean;
  desired_revision_generation: number;
  build_configuration: BuildConfiguration;
  configuration_version: number;
  command_policy_version: string;
  command_scan_result: CommandScanResult;
  created_at: string;
  updated_at: string;
}

export type DeploymentStatus = "pending" | "building" | "success" | "failed" | "canceled" | string;

export interface Deployment {
  id: string;
  user_id: string;
  repo_id: number;
  clone_url: string;
  status: DeploymentStatus;
  project_id?: string;
  github_installation_id?: string;
  commit_sha?: string;
  requested_ref?: string;
  trigger?: string;
  desired_revision_generation?: number;
  build_number?: number;
  configuration_snapshot?: BuildConfiguration;
  configuration_version?: number;
  command_policy_version?: string;
  command_scan_result?: CommandScanResult;
  manual_idempotency_key?: string;
  output_url?: string;
  error_message?: string;
  created_at: string;
  updated_at: string;
}

export interface GithubRepository {
  id: number;
  owner: string;
  name: string;
  full_name: string;
  default_branch: string;
  clone_url: string;
  private: boolean;
}

export interface GithubRepositoryList {
  repositories: GithubRepository[];
  next_cursor?: string;
}

export interface GithubInstallation {
  id: string;
  user_id: string;
  installation_id: number;
  account_type: string;
  account_login: string;
  status: string;
  created_at: string;
  updated_at: string;
}
