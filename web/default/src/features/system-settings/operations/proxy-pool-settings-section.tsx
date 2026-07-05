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
import * as z from 'zod'
import { useEffect, useMemo, useState } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SettingsSection } from '../components/settings-section'
import { getProxyPoolStatus, resetProxyPoolRuntime } from '../api'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import type { ProxyPoolStatus } from '../types'

type ProxyResource = {
  url: string
  enabled: boolean
}

const proxyPoolSchema = z.object({
  'proxy_pool_setting.enabled': z.boolean(),
  'proxy_pool_setting.proxies': z.string(),
  'proxy_pool_setting.health_check_url': z.string().url(),
  'proxy_pool_setting.health_check_interval_seconds': z.coerce
    .number()
    .int()
    .min(10),
  'proxy_pool_setting.health_check_timeout_seconds': z.coerce
    .number()
    .int()
    .min(1),
  'proxy_pool_setting.assignment_cooldown_seconds': z.coerce
    .number()
    .int()
    .min(0),
})

type ProxyPoolFormValues = z.infer<typeof proxyPoolSchema>

type ProxyPoolSettingsSectionProps = {
  defaultValues: ProxyPoolFormValues
}

function parseProxyResources(value: string): ProxyResource[] {
  try {
    const parsed = JSON.parse(value) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed
      .map((item) => {
        if (typeof item === 'string') {
          return { url: item.trim(), enabled: true }
        }
        if (item && typeof item === 'object') {
          const resource = item as Partial<ProxyResource>
          return {
            url: String(resource.url || '').trim(),
            enabled: resource.enabled !== false,
          }
        }
        return { url: '', enabled: true }
      })
      .filter((resource) => resource.url !== '')
  } catch {
    return []
  }
}

function resourcesToTextarea(value: string): string {
  return parseProxyResources(value)
    .filter((resource) => resource.enabled)
    .map((resource) => resource.url)
    .join('\n')
}

function textareaToResources(value: string): ProxyResource[] {
  return value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((url) => ({ url, enabled: true }))
}

function formatTimestamp(timestamp: number): string {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

function resourceStatusLabel(resource: ProxyPoolStatus['resources'][number]) {
  if (!resource.enabled) return { label: 'Disabled', variant: 'neutral' as const }
  if (!resource.available) return { label: 'Unavailable', variant: 'danger' as const }
  if (resource.cooldown_remaining_seconds > 0) {
    return { label: 'Cooling down', variant: 'warning' as const }
  }
  if (!resource.checked) return { label: 'Unchecked', variant: 'info' as const }
  return { label: 'Available', variant: 'success' as const }
}

export function ProxyPoolSettingsSection({
  defaultValues,
}: ProxyPoolSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [resettingRuntime, setResettingRuntime] = useState(false)
  const [statusLoading, setStatusLoading] = useState(false)
  const [proxyPoolStatus, setProxyPoolStatus] = useState<ProxyPoolStatus | null>(null)

  const formDefaults = useMemo(
    () => ({
      ...defaultValues,
      'proxy_pool_setting.proxies': resourcesToTextarea(
        defaultValues['proxy_pool_setting.proxies']
      ),
    }),
    [defaultValues]
  )

  const form = useForm<ProxyPoolFormValues>({
    resolver: zodResolver(proxyPoolSchema) as unknown as Resolver<ProxyPoolFormValues>,
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  const refreshProxyPoolStatus = async () => {
    setStatusLoading(true)
    try {
      const res = await getProxyPoolStatus()
      if (!res.success) throw new Error(res.message)
      setProxyPoolStatus(res.data)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to load proxy pool status')
      )
    } finally {
      setStatusLoading(false)
    }
  }

  useEffect(() => {
    void refreshProxyPoolStatus()
  }, [])

  const handleResetRuntime = async () => {
    setResettingRuntime(true)
    try {
      const res = await resetProxyPoolRuntime()
      if (!res.success) throw new Error(res.message)
      toast.success(t('Proxy pool cooldown reset'))
      await refreshProxyPoolStatus()
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to reset proxy pool cooldown')
      )
    } finally {
      setResettingRuntime(false)
    }
  }

  const onSubmit = async (values: ProxyPoolFormValues) => {
    const updates = [
      {
        key: 'proxy_pool_setting.enabled',
        value: values['proxy_pool_setting.enabled'],
      },
      {
        key: 'proxy_pool_setting.proxies',
        value: JSON.stringify(
          textareaToResources(values['proxy_pool_setting.proxies'])
        ),
      },
      {
        key: 'proxy_pool_setting.health_check_url',
        value: values['proxy_pool_setting.health_check_url'].trim(),
      },
      {
        key: 'proxy_pool_setting.health_check_interval_seconds',
        value: values['proxy_pool_setting.health_check_interval_seconds'],
      },
      {
        key: 'proxy_pool_setting.health_check_timeout_seconds',
        value: values['proxy_pool_setting.health_check_timeout_seconds'],
      },
      {
        key: 'proxy_pool_setting.assignment_cooldown_seconds',
        value: values['proxy_pool_setting.assignment_cooldown_seconds'],
      },
    ]

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
    await refreshProxyPoolStatus()
  }

  return (
    <SettingsSection title={t('Global Proxy Pool')}>
      <Form {...form}>
        <form
          onSubmit={form.handleSubmit(onSubmit)}
          autoComplete='off'
          className='space-y-6'
        >
          <FormField
            control={form.control}
            name='proxy_pool_setting.enabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                <div className='space-y-0.5'>
                  <FormLabel className='text-base'>
                    {t('Enable global proxy pool')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Channels can opt in to receive a monitored proxy from this shared pool.'
                    )}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='proxy_pool_setting.proxies'
            render={({ field }) => (
              <FormItem data-settings-form-span='full'>
                <FormLabel>{t('Proxy resources')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={6}
                    placeholder={t('One proxy URL per line')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Supports http, https, socks5, and socks5h proxy URLs.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='proxy_pool_setting.health_check_url'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Health check URL')}</FormLabel>
                <FormControl>
                  <Input type='url' inputMode='url' {...field} />
                </FormControl>
                <FormDescription>
                  {t('Each proxy is checked against this URL on a schedule.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='proxy_pool_setting.health_check_interval_seconds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Health check interval seconds')}</FormLabel>
                <FormControl>
                  <Input type='number' min={10} step={1} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='proxy_pool_setting.health_check_timeout_seconds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Health check timeout seconds')}</FormLabel>
                <FormControl>
                  <Input type='number' min={1} step={1} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='proxy_pool_setting.assignment_cooldown_seconds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Assignment cooldown seconds')}</FormLabel>
                <FormControl>
                  <Input type='number' min={0} step={1} {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'After a proxy is assigned, it must cool down before being assigned to another key.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='space-y-3' data-settings-form-span='full'>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <div>
                <h4 className='text-sm font-medium'>{t('Proxy pool status')}</h4>
                <p className='text-muted-foreground text-sm'>
                  {proxyPoolStatus
                    ? t('{{usable}} of {{total}} proxies are ready', {
                        usable: proxyPoolStatus.usable,
                        total: proxyPoolStatus.total,
                      })
                    : t('Current proxy runtime state')}
                </p>
              </div>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={statusLoading}
                onClick={() => void refreshProxyPoolStatus()}
              >
                {statusLoading ? t('Refreshing...') : t('Refresh status')}
              </Button>
            </div>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Proxy')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Assignments')}</TableHead>
                  <TableHead>{t('Cooldown')}</TableHead>
                  <TableHead>{t('Last check')}</TableHead>
                  <TableHead>{t('Last assigned')}</TableHead>
                  <TableHead>{t('Last error')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {!proxyPoolStatus?.resources.length ? (
                  <TableRow>
                    <TableCell colSpan={7} className='h-20 text-center'>
                      {statusLoading ? t('Loading...') : t('No proxy resources')}
                    </TableCell>
                  </TableRow>
                ) : (
                  proxyPoolStatus.resources.map((resource) => {
                    const status = resourceStatusLabel(resource)
                    return (
                      <TableRow key={resource.url}>
                        <TableCell className='max-w-[260px] truncate font-mono text-xs'>
                          {resource.name ? `${resource.name} ` : ''}
                          {resource.url}
                        </TableCell>
                        <TableCell>
                          <StatusBadge variant={status.variant} copyable={false}>
                            {t(status.label)}
                          </StatusBadge>
                        </TableCell>
                        <TableCell>{resource.assignment_count}</TableCell>
                        <TableCell>
                          {resource.cooldown_remaining_seconds > 0
                            ? t('{{seconds}}s remaining', {
                                seconds: resource.cooldown_remaining_seconds,
                              })
                            : '-'}
                        </TableCell>
                        <TableCell>{formatTimestamp(resource.last_checked_at)}</TableCell>
                        <TableCell>{formatTimestamp(resource.last_assigned_at)}</TableCell>
                        <TableCell className='max-w-[220px] truncate'>
                          {resource.last_error || '-'}
                        </TableCell>
                      </TableRow>
                    )
                  })
                )}
              </TableBody>
            </Table>
          </div>

          <div className='flex flex-wrap gap-2'>
            <Button type='submit' disabled={updateOption.isPending || resettingRuntime}>
              {t('Save proxy pool settings')}
            </Button>
            <Button
              type='button'
              variant='outline'
              disabled={updateOption.isPending || resettingRuntime}
              onClick={() => void handleResetRuntime()}
            >
              {resettingRuntime
                ? t('Resetting...')
                : t('Reset proxy pool cooldown')}
            </Button>
          </div>
        </form>
      </Form>
    </SettingsSection>
  )
}
