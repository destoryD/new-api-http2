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
import { useState, useEffect, useCallback } from 'react'
import { useQueryClient, useIsFetching } from '@tanstack/react-query'
import { useNavigate, getRouteApi } from '@tanstack/react-router'
import { type Table } from '@tanstack/react-table'
import { Download, Eye, EyeOff } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useIsAdmin } from '@/hooks/use-admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DataTableToolbar } from '@/components/data-table'
import { LOG_TYPES } from '../constants'
import {
  downloadBillingLogs,
  type BillingExportFormat,
  type BillingExportKind,
} from '../lib/export'
import { buildSearchParams } from '../lib/filter'
import { buildApiParams, getDefaultTimeRange } from '../lib/utils'
import type { CommonLogFilters } from '../types'
import { CommonLogsStats } from './common-logs-stats'
import { CompactDateTimeRangePicker } from './compact-date-time-range-picker'
import { useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')
const logTypeValues = ['0', '1', '2', '3', '4', '5', '6'] as const

type LogTypeValue = (typeof logTypeValues)[number]

function isLogTypeValue(value: string): value is LogTypeValue {
  return (logTypeValues as readonly string[]).includes(value)
}

interface CommonLogsFilterBarProps<TData> {
  table: Table<TData>
}

export function CommonLogsFilterBar<TData>(
  props: CommonLogsFilterBarProps<TData>
) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const searchParams = route.useSearch()
  const isAdmin = useIsAdmin()
  const { sensitiveVisible, setSensitiveVisible } = useUsageLogsContext()
  const fetchingLogs = useIsFetching({ queryKey: ['logs'] })
  const [exportFormat, setExportFormat] = useState<BillingExportFormat>('csv')
  const [exportingKind, setExportingKind] = useState<BillingExportKind | null>(
    null
  )

  const [filters, setFilters] = useState<CommonLogFilters>(() => {
    const { start, end } = getDefaultTimeRange()
    return { startTime: start, endTime: end }
  })
  const [logType, setLogType] = useState<LogTypeValue | ''>('')

  useEffect(() => {
    const next: Partial<CommonLogFilters> = {}
    if (searchParams.startTime)
      next.startTime = new Date(searchParams.startTime)
    if (searchParams.endTime) next.endTime = new Date(searchParams.endTime)
    if (searchParams.channel) next.channel = String(searchParams.channel)
    if (searchParams.model) next.model = searchParams.model
    if (searchParams.token) next.token = searchParams.token
    if (searchParams.group) next.group = searchParams.group
    if (searchParams.username) next.username = searchParams.username
    if (searchParams.requestId) next.requestId = searchParams.requestId
    if (searchParams.upstreamRequestId)
      next.upstreamRequestId = searchParams.upstreamRequestId

    if (Object.keys(next).length > 0) {
      setFilters((prev) => ({ ...prev, ...next }))
    }

    const typeArr = searchParams.type
    if (Array.isArray(typeArr) && typeArr.length === 1) {
      setLogType(typeArr[0])
    }
  }, [
    searchParams.startTime,
    searchParams.endTime,
    searchParams.channel,
    searchParams.model,
    searchParams.token,
    searchParams.group,
    searchParams.username,
    searchParams.requestId,
    searchParams.upstreamRequestId,
    searchParams.type,
  ])

  const handleChange = useCallback(
    (field: keyof CommonLogFilters, value: Date | string | undefined) => {
      setFilters((prev) => ({ ...prev, [field]: value }))
    },
    []
  )

  const handleApply = useCallback(() => {
    const filterParams = buildSearchParams(filters, 'common')
    navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        ...filterParams,
        ...(logType ? { type: [logType] } : {}),
        page: 1,
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
    queryClient.invalidateQueries({ queryKey: ['usage-logs-stats'] })
  }, [filters, logType, navigate, queryClient])

  const handleReset = useCallback(() => {
    const { start, end } = getDefaultTimeRange()
    const resetFilters: CommonLogFilters = { startTime: start, endTime: end }
    setFilters(resetFilters)
    setLogType('')

    navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        page: 1,
        startTime: start.getTime(),
        endTime: end.getTime(),
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
    queryClient.invalidateQueries({ queryKey: ['usage-logs-stats'] })
  }, [navigate, queryClient])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') handleApply()
    },
    [handleApply]
  )

  const handleExport = useCallback(
    async (kind: BillingExportKind = 'detail') => {
      setExportingKind(kind)
      try {
        const params = buildApiParams({
          page: 1,
          pageSize: 100,
          searchParams,
          columnFilters: props.table.getState().columnFilters,
          isAdmin,
        })
        await downloadBillingLogs(params, isAdmin, exportFormat, kind)
        toast.success(
          t(
            kind === 'reconciliation'
              ? 'Billing reconciliation exported'
              : 'Billing logs exported'
          )
        )
      } catch (error) {
        toast.error(
          error instanceof Error ? error.message : t('Failed to export logs')
        )
      } finally {
        setExportingKind(null)
      }
    },
    [exportFormat, isAdmin, props.table, searchParams, t]
  )

  const hasExpandedFilters =
    !!filters.token ||
    !!filters.username ||
    !!filters.channel ||
    !!filters.requestId ||
    !!filters.upstreamRequestId

  const hasAdditionalFilters =
    !!filters.model || !!filters.group || !!logType || hasExpandedFilters

  const inputClass = 'w-full sm:w-[140px] lg:w-[160px]'
  const sensitiveType = sensitiveVisible ? 'text' : 'password'

  const statsBar = (
    <div className='flex flex-wrap items-center gap-2'>
      <CommonLogsStats />
      <Button
        variant='outline'
        size='sm'
        disabled={exportingKind !== null}
        onClick={() => void handleExport('detail')}
        className='h-7 gap-1.5 px-2'
      >
        <Download className='h-3.5 w-3.5' />
        <span className='whitespace-nowrap'>
          {exportingKind === 'detail' ? t('Exporting') : t('Export Details')}
        </span>
      </Button>
      <Button
        variant='outline'
        size='sm'
        disabled={exportingKind !== null}
        onClick={() => void handleExport('reconciliation')}
        className='h-7 gap-1.5 px-2'
      >
        <Download className='h-3.5 w-3.5' />
        <span className='whitespace-nowrap'>
          {exportingKind === 'reconciliation'
            ? t('Exporting')
            : t('Export Reconciliation')}
        </span>
      </Button>
      <Select
        items={[
          { value: 'csv', label: 'CSV' },
          { value: 'txt', label: 'TXT' },
        ]}
        value={exportFormat}
        onValueChange={(value) => {
          if (value === 'csv' || value === 'txt') setExportFormat(value)
        }}
        disabled={exportingKind !== null}
      >
        <SelectTrigger
          aria-label={t('File Type')}
          className='h-7 w-[78px] px-2 text-xs'
        >
          <SelectValue placeholder='CSV' />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            <SelectItem value='csv'>CSV</SelectItem>
            <SelectItem value='txt'>TXT</SelectItem>
          </SelectGroup>
        </SelectContent>
      </Select>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon'
              onClick={() => setSensitiveVisible(!sensitiveVisible)}
              aria-label={sensitiveVisible ? t('Hide') : t('Show')}
              className='text-muted-foreground hover:text-foreground size-7'
            />
          }
        >
          {sensitiveVisible ? <Eye /> : <EyeOff />}
        </TooltipTrigger>
        <TooltipContent>
          {sensitiveVisible ? t('Hide') : t('Show')}
        </TooltipContent>
      </Tooltip>
    </div>
  )

  return (
    <DataTableToolbar
      table={props.table}
      leftActions={statsBar}
      customSearch={
        <CompactDateTimeRangePicker
          start={filters.startTime}
          end={filters.endTime}
          onChange={({ start, end }) => {
            handleChange('startTime', start)
            handleChange('endTime', end)
          }}
          className='w-full sm:w-[340px]'
        />
      }
      additionalSearch={
        <>
          <Input
            placeholder={t('Model Name')}
            value={filters.model || ''}
            onChange={(e) => handleChange('model', e.target.value)}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
          <Input
            placeholder={t('Group')}
            type={sensitiveType}
            value={filters.group || ''}
            onChange={(e) => handleChange('group', e.target.value)}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
          <Select
            items={[
              { value: 'all', label: t('All Types') },
              ...LOG_TYPES.map((type) => ({
                value: String(type.value),
                label: t(type.label),
              })),
            ]}
            value={logType}
            onValueChange={(value) => {
              setLogType(value !== null && isLogTypeValue(value) ? value : '')
            }}
          >
            <SelectTrigger className={inputClass}>
              <SelectValue placeholder={t('All Types')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='all'>{t('All Types')}</SelectItem>
                {LOG_TYPES.map((type) => (
                  <SelectItem key={type.value} value={String(type.value)}>
                    {t(type.label)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </>
      }
      expandable={
        <>
          <Input
            placeholder={t('Token Name')}
            type={sensitiveType}
            value={filters.token || ''}
            onChange={(e) => handleChange('token', e.target.value)}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
          {isAdmin && (
            <Input
              placeholder={t('Username')}
              type={sensitiveType}
              value={filters.username || ''}
              onChange={(e) => handleChange('username', e.target.value)}
              onKeyDown={handleKeyDown}
              className={inputClass}
            />
          )}
          {isAdmin && (
            <Input
              placeholder={t('Channel ID')}
              value={filters.channel || ''}
              onChange={(e) => handleChange('channel', e.target.value)}
              onKeyDown={handleKeyDown}
              className={inputClass}
            />
          )}
          <Input
            placeholder={t('Request ID')}
            value={filters.requestId || ''}
            onChange={(e) => handleChange('requestId', e.target.value)}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
          <Input
            placeholder={t('Upstream Request ID')}
            value={filters.upstreamRequestId || ''}
            onChange={(e) => handleChange('upstreamRequestId', e.target.value)}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
        </>
      }
      hasExpandedActiveFilters={hasExpandedFilters}
      hasAdditionalFilters={hasAdditionalFilters}
      onSearch={handleApply}
      searchLoading={fetchingLogs > 0}
      onReset={handleReset}
    />
  )
}
