import { PaginationRequest, PaginationResponse } from '@/core/api/pagination'
import { Workspace, WorkspaceApi, WorkspaceModel } from '@/core/api/workspace'
import type { NextApiRequest, NextApiResponse } from 'next'

const WorkspaceApiMock: WorkspaceApi = {
  list: function (request: PaginationRequest): PaginationResponse<WorkspaceModel[]> {
    throw new Error('Function not implemented.')
  },
  get: function (id: string): WorkspaceModel {
    throw new Error('Function not implemented.')
  },
  create: function (workspace: Workspace): WorkspaceModel {
    throw new Error('Function not implemented.')
  },
  update: function (workspace: Workspace): WorkspaceModel {
    throw new Error('Function not implemented.')
  },
  delete: function (id: string): void {
    throw new Error('Function not implemented.')
  }
}
