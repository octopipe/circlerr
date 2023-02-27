export interface PaginationRequest {
  limit: number
  continue: string
}

export interface PaginationResponse<T> {
  limit: number
  continue: string
  items: T
}