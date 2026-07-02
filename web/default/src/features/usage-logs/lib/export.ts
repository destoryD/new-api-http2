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
import { getAllLogs, getUserLogs } from '../api'
import { LOG_TYPES } from '../constants'
import type { UsageLog } from '../data/schema'
import type { GetLogsParams } from '../types'

export type BillingExportFormat = 'csv' | 'txt'

const EXPORT_PAGE_SIZE = 100

const LOG_TYPE_LABELS = new Map<number, string>(
  LOG_TYPES.map((type) => [type.value, type.label])
)

type Translate = (key: string) => string

const BILLING_EXPORT_COLUMNS: Array<{
  key: string
  label: string
  value: (log: UsageLog, t: Translate) => unknown
}> = [
  {
    key: 'id',
    label: 'ID',
    value: (log) => log.id,
  },
  {
    key: 'created_at',
    label: 'Time',
    value: (log) => formatTimestamp(log.created_at),
  },
  {
    key: 'type',
    label: 'Type',
    value: (log, t) => t(LOG_TYPE_LABELS.get(log.type) || 'Unknown'),
  },
  {
    key: 'username',
    label: 'Username',
    value: (log) => log.username,
  },
  {
    key: 'user_id',
    label: 'User ID',
    value: (log) => log.user_id,
  },
  {
    key: 'token_name',
    label: 'Token Name',
    value: (log) => log.token_name,
  },
  {
    key: 'token_id',
    label: 'Token ID',
    value: (log) => log.token_id,
  },
  {
    key: 'model_name',
    label: 'Model Name',
    value: (log) => log.model_name,
  },
  {
    key: 'channel',
    label: 'Channel ID',
    value: (log) => log.channel,
  },
  {
    key: 'channel_name',
    label: 'Channel',
    value: (log) => log.channel_name,
  },
  {
    key: 'group',
    label: 'Group',
    value: (log) => log.group,
  },
  {
    key: 'quota',
    label: 'Quota',
    value: (log) => log.quota,
  },
  {
    key: 'prompt_tokens',
    label: 'Prompt Tokens',
    value: (log) => log.prompt_tokens,
  },
  {
    key: 'completion_tokens',
    label: 'Completion Tokens',
    value: (log) => log.completion_tokens,
  },
  {
    key: 'total_tokens',
    label: 'Total Tokens',
    value: (log) => log.prompt_tokens + log.completion_tokens,
  },
  {
    key: 'use_time',
    label: 'Duration',
    value: (log) => log.use_time,
  },
  {
    key: 'is_stream',
    label: 'Stream',
    value: (log) => (log.is_stream ? 'true' : 'false'),
  },
  {
    key: 'request_id',
    label: 'Request ID',
    value: (log) => log.request_id,
  },
  {
    key: 'upstream_request_id',
    label: 'Upstream Request ID',
    value: (log) => log.upstream_request_id,
  },
  {
    key: 'ip',
    label: 'IP',
    value: (log) => log.ip,
  },
  {
    key: 'content',
    label: 'Content',
    value: (log) => log.content,
  },
  {
    key: 'other',
    label: 'Details',
    value: (log) => normalizeJsonString(log.other),
  },
]

export async function fetchBillingExportLogs(
  params: GetLogsParams,
  isAdmin: boolean
): Promise<UsageLog[]> {
  const logs: UsageLog[] = []
  let page = 1
  let total = 0

  do {
    const res = isAdmin
      ? await getAllLogs({ ...params, p: page, page_size: EXPORT_PAGE_SIZE })
      : await getUserLogs({ ...params, p: page, page_size: EXPORT_PAGE_SIZE })

    if (!res.success) {
      throw new Error(res.message || 'Failed to export billing logs')
    }

    const data = res.data
    const items = (data?.items || []) as UsageLog[]
    total = data?.total || items.length
    logs.push(...items)

    if (items.length < EXPORT_PAGE_SIZE) break
    page += 1
  } while (logs.length < total)

  return logs
}

export function downloadBillingLogs(
  logs: UsageLog[],
  format: BillingExportFormat,
  t: Translate
) {
  const content =
    format === 'csv'
      ? '\ufeff' + buildSeparatedValues(logs, ',', t)
      : buildSeparatedValues(logs, '\t', t)
  const mimeType =
    format === 'csv' ? 'text/csv;charset=utf-8' : 'text/plain;charset=utf-8'
  const filename = `billing-logs-${formatDateForFilename(new Date())}.${format}`
  downloadTextFile(content, filename, mimeType)
}

function buildSeparatedValues(
  logs: UsageLog[],
  delimiter: string,
  t: Translate
) {
  const headers = BILLING_EXPORT_COLUMNS.map((column) => t(column.label))
  const rows = logs.map((log) =>
    BILLING_EXPORT_COLUMNS.map((column) =>
      escapeSeparatedValue(column.value(log, t), delimiter)
    ).join(delimiter)
  )
  return [
    headers
      .map((header) => escapeSeparatedValue(header, delimiter))
      .join(delimiter),
    ...rows,
  ].join('\n')
}

function escapeSeparatedValue(value: unknown, delimiter: string): string {
  const str = value == null ? '' : String(value)
  const normalized = str.replace(/\r?\n/g, ' ')
  if (
    delimiter === ',' &&
    (normalized.includes(',') ||
      normalized.includes('"') ||
      normalized.includes('\n') ||
      normalized.includes('\r'))
  ) {
    return `"${normalized.replace(/"/g, '""')}"`
  }
  if (delimiter === '\t') {
    return normalized.replace(/\t/g, ' ')
  }
  return normalized
}

function formatTimestamp(timestamp: number): string {
  if (!timestamp) return ''
  return new Date(timestamp * 1000).toLocaleString()
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

function normalizeJsonString(value: string): string {
  if (!value) return ''
  try {
    return JSON.stringify(JSON.parse(value))
  } catch {
    return value
  }
}

function downloadTextFile(content: string, filename: string, mimeType: string) {
  const blob = new Blob([content], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}
