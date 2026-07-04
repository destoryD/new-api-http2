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

export async function downloadBillingLogs(
  params: GetLogsParams,
  isAdmin: boolean,
  format: BillingExportFormat,
  kind: BillingExportKind = 'detail'
) {
  const queryParams = buildQueryParams({ ...params, format })
  const endpoint =
    kind === 'reconciliation'
      ? isAdmin
        ? '/api/log/reconciliation_export'
        : '/api/log/self/reconciliation_export'
      : isAdmin
        ? '/api/log/export'
        : '/api/log/self/export'
  const res = await api.get(`${endpoint}?${queryParams}`, {
    responseType: 'blob',
    disableDuplicate: true,
    skipBusinessError: true,
  } as Record<string, unknown>)

  const filename =
    getFilenameFromDisposition(res.headers['content-disposition']) ||
    `${kind === 'reconciliation' ? 'billing-reconciliation' : 'billing-logs'}-${formatDateForFilename(new Date())}.${format}`
  downloadBlob(res.data as Blob, filename)
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
  const match = disposition.match(/filename="?([^"]+)"?/i)
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
