/*
Copyright (C) 2025 QuantumNous

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

import React, { useEffect, useRef, useState } from 'react';
import { Button, Col, Form, Row, Spin, Table, Tag, Typography } from '@douyinfe/semi-ui';
import {
  API,
  compareObjects,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

const parseProxyResources = (value) => {
  if (!value || !String(value).trim()) {
    return [];
  }
  try {
    const parsed = JSON.parse(value);
    if (Array.isArray(parsed)) {
      return parsed
        .map((proxy) => {
          if (typeof proxy === 'string') {
            return { name: '', url: proxy.trim(), enabled: true };
          }
          if (proxy && typeof proxy === 'object') {
            return {
              name: String(proxy.name || ''),
              url: String(proxy.url || proxy.URL || '').trim(),
              enabled: proxy.enabled !== false && proxy.Enabled !== false,
            };
          }
          return null;
        })
        .filter((proxy) => proxy && proxy.url);
    }
  } catch (error) {
    return String(value)
      .split(/\r?\n/)
      .map((url) => url.trim())
      .filter(Boolean)
      .map((url) => ({ name: '', url, enabled: true }));
  }
  return [];
};

const formatProxyResources = (value) => {
  return parseProxyResources(value)
    .map((proxy) => proxy.url)
    .join('\n');
};

const buildProxyResources = (value, previousValue) => {
  const previousByURL = new Map(
    parseProxyResources(previousValue).map((proxy) => [proxy.url, proxy]),
  );
  return Array.from(
    new Set(
      String(value || '')
        .split(/\r?\n/)
        .map((url) => url.trim())
        .filter(Boolean),
    ),
  ).map((url, index) => {
    const previous = previousByURL.get(url);
    return {
      name: previous?.name || `proxy-${index + 1}`,
      url,
      enabled: previous?.enabled !== false,
    };
  });
};

export default function SettingsProxyPool(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [resettingRuntime, setResettingRuntime] = useState(false);
  const [statusLoading, setStatusLoading] = useState(false);
  const [proxyPoolStatus, setProxyPoolStatus] = useState(null);
  const [rawProxyResources, setRawProxyResources] = useState('');
  const [inputs, setInputs] = useState({
    'proxy_pool_setting.enabled': false,
    'proxy_pool_setting.proxies': '',
    'proxy_pool_setting.health_check_url': 'https://api.openai.com',
    'proxy_pool_setting.health_check_interval_seconds': 300,
    'proxy_pool_setting.health_check_timeout_seconds': 10,
    'proxy_pool_setting.assignment_cooldown_seconds': 60,
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);

  function handleFieldChange(fieldName) {
    return (value) => {
      setInputs((inputs) => ({ ...inputs, [fieldName]: value }));
    };
  }

  function fetchProxyPoolStatus() {
    setStatusLoading(true);
    API.get('/api/option/proxy_pool/status')
      .then((res) => {
        if (!res.data.success) {
          return showError(res.data.message || t('Failed to load proxy pool status'));
        }
        setProxyPoolStatus(res.data.data);
      })
      .catch(() => {
        showError(t('Failed to load proxy pool status'));
      })
      .finally(() => {
        setStatusLoading(false);
      });
  }

  function resetProxyPoolRuntime() {
    setResettingRuntime(true);
    API.post('/api/option/proxy_pool/reset_runtime')
      .then((res) => {
        if (!res.data.success) {
          return showError(res.data.message || t('重置代理池冷却失败'));
        }
        showSuccess(t('代理池冷却已重置'));
      })
      .catch(() => {
        showError(t('重置代理池冷却失败'));
      })
      .finally(() => {
        setResettingRuntime(false);
        fetchProxyPoolStatus();
      });
  }

  function onSubmit() {
    const submitInputs = {
      ...inputs,
      'proxy_pool_setting.proxies': JSON.stringify(
        buildProxyResources(
          inputs['proxy_pool_setting.proxies'],
          rawProxyResources,
        ),
      ),
    };
    const submitInputsRow = {
      ...inputsRow,
      'proxy_pool_setting.proxies': JSON.stringify(
        buildProxyResources(
          inputsRow['proxy_pool_setting.proxies'],
          rawProxyResources,
        ),
      ),
    };
    const updateArray = compareObjects(submitInputs, submitInputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    const requestQueue = updateArray.map((item) => {
      const value =
        typeof submitInputs[item.key] === 'boolean'
          ? String(submitInputs[item.key])
          : String(submitInputs[item.key]);
      return API.put('/api/option/', {
        key: item.key,
        value,
      });
    });
    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (requestQueue.length === 1) {
          if (res.includes(undefined)) return;
        } else if (requestQueue.length > 1) {
          if (res.includes(undefined))
            return showError(t('部分保存失败，请重试'));
        }
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  useEffect(() => {
    fetchProxyPoolStatus();
  }, []);

  useEffect(() => {
    const currentInputs = {};
    for (let key in props.options) {
      if (Object.keys(inputs).includes(key)) {
        currentInputs[key] = props.options[key];
      }
    }
    const rawProxies = currentInputs['proxy_pool_setting.proxies'] || '[]';
    currentInputs['proxy_pool_setting.proxies'] =
      formatProxyResources(rawProxies);
    setRawProxyResources(rawProxies);
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    refForm.current.setValues(currentInputs);
  }, [props.options]);

  const formatTimestamp = (timestamp) => {
    if (!timestamp) return '-';
    return new Date(timestamp * 1000).toLocaleString();
  };

  const renderStatusTag = (resource) => {
    if (!resource.enabled) return <Tag color='grey'>{t('Disabled')}</Tag>;
    if (!resource.available) return <Tag color='red'>{t('Unavailable')}</Tag>;
    if (resource.cooldown_remaining_seconds > 0) {
      return <Tag color='orange'>{t('Cooling down')}</Tag>;
    }
    if (!resource.checked) return <Tag color='blue'>{t('Unchecked')}</Tag>;
    return <Tag color='green'>{t('Available')}</Tag>;
  };

  const proxyPoolStatusColumns = [
    {
      title: t('Proxy'),
      dataIndex: 'url',
      render: (url, record) => (
        <Typography.Text ellipsis={{ showTooltip: true }} style={{ maxWidth: 260 }}>
          {record.name ? record.name + ' ' : ''}{url}
        </Typography.Text>
      ),
    },
    { title: t('Status'), dataIndex: 'available', render: (_, record) => renderStatusTag(record) },
    { title: t('Assignments'), dataIndex: 'assignment_count' },
    {
      title: t('Cooldown'),
      dataIndex: 'cooldown_remaining_seconds',
      render: (seconds) => (seconds > 0 ? t('{{seconds}}s remaining', { seconds }) : '-'),
    },
    { title: t('Last check'), dataIndex: 'last_checked_at', render: formatTimestamp },
    { title: t('Last assigned'), dataIndex: 'last_assigned_at', render: formatTimestamp },
    { title: t('Last error'), dataIndex: 'last_error', render: (value) => value || '-' },
  ];

  return (
    <>
      <Spin spinning={loading || resettingRuntime}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('全局代理池')}>
            <Typography.Text
              type='tertiary'
              style={{ marginBottom: 16, display: 'block' }}
            >
              {t('渠道可选择从此共享池接收受监测的代理。')}
            </Typography.Text>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'proxy_pool_setting.enabled'}
                  label={t('启用全局代理池')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={handleFieldChange('proxy_pool_setting.enabled')}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Input
                  field={'proxy_pool_setting.health_check_url'}
                  label={t('健康检测 URL')}
                  placeholder='https://api.openai.com'
                  extraText={t('系统会按计划使用此 URL 检测每个代理。')}
                  onChange={handleFieldChange(
                    'proxy_pool_setting.health_check_url',
                  )}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'proxy_pool_setting.health_check_interval_seconds'}
                  label={t('健康检测间隔秒数')}
                  min={10}
                  step={1}
                  suffix={t('秒')}
                  onChange={handleFieldChange(
                    'proxy_pool_setting.health_check_interval_seconds',
                  )}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'proxy_pool_setting.health_check_timeout_seconds'}
                  label={t('健康检测超时秒数')}
                  min={1}
                  step={1}
                  suffix={t('秒')}
                  onChange={handleFieldChange(
                    'proxy_pool_setting.health_check_timeout_seconds',
                  )}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field={'proxy_pool_setting.assignment_cooldown_seconds'}
                  label={t('分配冷却秒数')}
                  min={0}
                  step={1}
                  suffix={t('秒')}
                  extraText={t(
                    '代理分配后，必须等待冷却结束才能分配给另一个 key。',
                  )}
                  onChange={handleFieldChange(
                    'proxy_pool_setting.assignment_cooldown_seconds',
                  )}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={24}>
                <Form.TextArea
                  field={'proxy_pool_setting.proxies'}
                  label={t('代理资源')}
                  placeholder={t('每行一个代理 URL')}
                  autosize={{ minRows: 6, maxRows: 12 }}
                  showClear
                  extraText={t('支持 http、https、socks5 和 socks5h 代理 URL。')}
                  onChange={handleFieldChange('proxy_pool_setting.proxies')}
                />
              </Col>
            </Row>
            <div style={{ marginBottom: 16 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center', marginBottom: 8 }}>
                <div>
                  <Typography.Text strong>{t('Proxy pool status')}</Typography.Text>
                  <br />
                  <Typography.Text type='tertiary'>
                    {proxyPoolStatus
                      ? t('{{usable}} of {{total}} proxies are ready', { usable: proxyPoolStatus.usable, total: proxyPoolStatus.total })
                      : t('Current proxy runtime state')}
                  </Typography.Text>
                </div>
                <Button size='small' theme='outline' loading={statusLoading} onClick={fetchProxyPoolStatus}>
                  {t('Refresh status')}
                </Button>
              </div>
              <Table
                size='small'
                pagination={false}
                loading={statusLoading}
                columns={proxyPoolStatusColumns}
                dataSource={(proxyPoolStatus?.resources || []).map((item) => ({ ...item, key: item.url }))}
                empty={t('No proxy resources')}
              />
            </div>
            <Row>
              <Button size='default' onClick={onSubmit} disabled={resettingRuntime}>
                {t('保存全局代理池设置')}
              </Button>
              <Button
                size='default'
                theme='outline'
                style={{ marginLeft: 8 }}
                loading={resettingRuntime}
                onClick={resetProxyPoolRuntime}
              >
                {t('重置代理池冷却')}
              </Button>
            </Row>
          </Form.Section>
        </Form>
      </Spin>
    </>
  );
}
