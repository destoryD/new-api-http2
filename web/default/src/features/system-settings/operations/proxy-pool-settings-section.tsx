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
import { useMemo } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
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
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

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

export function ProxyPoolSettingsSection({
  defaultValues,
}: ProxyPoolSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

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

          <Button type='submit' disabled={updateOption.isPending}>
            {t('Save proxy pool settings')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
