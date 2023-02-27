import { PaginationResponse } from "@/core/api/pagination"
import { WorkspaceModel } from "@/core/api/workspace"

interface Props {
  pagination: PaginationResponse<WorkspaceModel[]>
}

export async function getServerSideProps() {
  const res = await fetch(`${process.env.API_URL}/workspaces`)
  const pagination = await res.json()

  return { props: { pagination } }
}


export default function Workspace({ pagination }: Props) {
  return (
    <ul>
      {pagination.items?.map(workspace => (
        <li>{workspace.name}</li>
      ))}
    </ul>
  )
}