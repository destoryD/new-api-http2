/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'
import type { GetLogsParams } from '../types'
import { buildQueryParams } from './utils'

export type BillingExportFormat = 'csv' | 'txt'
export type BillingExportKind = 'detail' | 'reconciliation'
export type BillingExportTaskStatus =
  'pending' | 'running' | 'success' | 'failed' | 'canceled'

export interface BillingExportTask {
  id: number
  created_at: number
  updated_at: number
  finished_at: number
  user_id: number
  username: string
  is_admin: boolean
  kind: BillingExportKind
  format: BillingExportFormat
  status: BillingExportTaskStatus
  progress: number
  rows: number
  filename: string
  file_size: number
  error: string
}

interface ApiResponse<T> {
  success: boolean
  message: string
  data: T
}

export async function createBillingLogExportTask(
  params: GetLogsParams,
  isAdmin: boolean,
  format: BillingExportFormat,
  kind: BillingExportKind = 'detail'
) {
  const queryParams = buildQueryParams({ ...params, format, kind })
  const endpoint = isAdmin
    ? '/api/log/export_tasks'
    : '/api/log/self/export_tasks'
  const res = await api.post<ApiResponse<BillingExportTask>>(
    endpoint + '?' + queryParams,
    null,
    { skipBusinessError: true } as Record<string, unknown>
  )
  if (!res.data.success) throw new Error(res.data.message)
  return res.data.data
}

export async function listBillingLogExportTasks(isAdmin: boolean) {
  const endpoint = isAdmin
    ? '/api/log/export_tasks'
    : '/api/log/self/export_tasks'
  const res = await api.get<ApiResponse<BillingExportTask[]>>(
    endpoint + '?limit=20',
    { disableDuplicate: true, skipBusinessError: true } as Record<
      string,
      unknown
    >
  )
  if (!res.data.success) throw new Error(res.data.message)
  return res.data.data
}

export async function downloadBillingLogExportTask(
  task: BillingExportTask,
  isAdmin: boolean
) {
  const endpoint = isAdmin
    ? '/api/log/export_tasks/' + task.id + '/download'
    : '/api/log/self/export_tasks/' + task.id + '/download'
  const res = await api.get(endpoint, {
    responseType: 'blob',
    disableDuplicate: true,
    skipBusinessError: true,
  } as Record<string, unknown>)
  const prefix =
    task.kind === 'reconciliation' ? 'billing-reconciliation' : 'billing-logs'
  const filename =
    getFilenameFromDisposition(res.headers['content-disposition']) ||
    task.filename ||
    prefix + '-' + formatDateForFilename(new Date()) + '.' + task.format
  downloadBlob(res.data as Blob, filename)
}

export async function cancelBillingLogExportTask(
  task: BillingExportTask,
  isAdmin: boolean
) {
  const endpoint = isAdmin
    ? '/api/log/export_tasks/' + task.id + '/cancel'
    : '/api/log/self/export_tasks/' + task.id + '/cancel'
  const res = await api.post<ApiResponse<null>>(endpoint, null, {
    disableDuplicate: true,
    skipBusinessError: true,
  } as Record<string, unknown>)
  if (!res.data.success) throw new Error(res.data.message)
}
export async function deleteBillingLogExportTask(
  task: BillingExportTask,
  isAdmin: boolean
) {
  const endpoint = isAdmin
    ? '/api/log/export_tasks/' + task.id
    : '/api/log/self/export_tasks/' + task.id
  const res = await api.delete<ApiResponse<null>>(endpoint, {
    disableDuplicate: true,
    skipBusinessError: true,
  } as Record<string, unknown>)
  if (!res.data.success) throw new Error(res.data.message)
}

function formatDateForFilename(date: Date): string {
  const pad = (value: number) => String(value).padStart(2, '0')
  return [
    date.getFullYear(),
    pad(date.getMonth() + 1),
    pad(date.getDate()),
    '-',
    pad(date.getHours()),
    pad(date.getMinutes()),
    pad(date.getSeconds()),
  ].join('')
}

function getFilenameFromDisposition(disposition?: string): string | null {
  if (!disposition) return null
  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match?.[1]) return decodeURIComponent(utf8Match[1])
  const match = disposition.match(/filename="?([^";]+)"?/i)
  return match?.[1] || null
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}
