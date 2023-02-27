import { PaginationRequest, PaginationResponse } from "./pagination"

export enum WORKSPACE_TYPE {
  DEFAULT = 'DEFAULT',
  CIRCLE = 'CIRCLE',
  CANARY = 'CANARY'
}

export interface Workspace {
  name: string
  description: string
  type: WORKSPACE_TYPE
}

export interface WorkspaceModel extends Workspace {
  id: string
  createdAt: string
}

export interface WorkspaceApi {
  list(request: PaginationRequest): PaginationResponse<WorkspaceModel[]>
  get(id: string): WorkspaceModel
  create(workspace: Workspace): WorkspaceModel
  update(workspace: Workspace): WorkspaceModel
  delete(id: string): void
}