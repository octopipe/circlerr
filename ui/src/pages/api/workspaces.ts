// Next.js API route support: https://nextjs.org/docs/api-routes/introduction
import { PaginationResponse } from '@/core/api/pagination'
import { WorkspaceModel, WORKSPACE_TYPE } from '@/core/api/workspace'
import type { NextApiRequest, NextApiResponse } from 'next'

export default function handler(
  req: NextApiRequest,
  res: NextApiResponse<PaginationResponse<WorkspaceModel[]>>
) {
  res.status(200).json({
    limit: 10,
    continue: 'asasas',
    items: [
      { id: 'id-1', name: 'workspace1', description: 'Lorem ipsum', type: WORKSPACE_TYPE.DEFAULT, createdAt: 'yesterday' }
    ]
  })
}
