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

import React, { useEffect, useState } from 'react';
import {
  Button,
  Tag,
  Space,
  Skeleton,
  Select,
  Modal,
  Progress,
} from '@douyinfe/semi-ui';
import { renderQuota } from '../../../helpers';
import CompactModeToggle from '../../common/ui/CompactModeToggle';
import { useMinimumLoadingTime } from '../../../hooks/common/useMinimumLoadingTime';

const textQuotaUsed = '\u6d88\u8017\u989d\u5ea6';
const textExportDetails = '\u5bfc\u51fa\u660e\u7ec6';
const textExportReconciliation = '\u5bfc\u51fa\u5bf9\u8d26\u5355';
const textFileType = '\u6587\u4ef6\u7c7b\u578b';
const textDownloadCenter = '\u4e0b\u8f7d\u4e2d\u5fc3';
const textRefresh = '\u5237\u65b0';
const textNoExportTasks = '\u6682\u65e0\u5bfc\u51fa\u4efb\u52a1';
const textDownload = '\u4e0b\u8f7d';
const textDelete = '\u5220\u9664';
const textRows = '\u884c\u6570';
const textSize = '\u5927\u5c0f';
const textCreatedAt = '\u521b\u5efa\u65f6\u95f4';

const formatExportTime = (timestamp) => {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
};

const formatExportSize = (size) => {
  if (!size) return '-';
  if (size < 1024) return size + ' B';
  if (size < 1024 * 1024) return (size / 1024).toFixed(1) + ' KB';
  return (size / 1024 / 1024).toFixed(1) + ' MB';
};

const statusColor = (status) => {
  if (status === 'success') return 'green';
  if (status === 'failed') return 'red';
  return 'blue';
};

const LogsActions = ({
  stat,
  loadingStat,
  showStat,
  compactMode,
  setCompactMode,
  exportingFormat,
  exportBillingLogs,
  exportTasks,
  loadingExportTasks,
  loadExportTasks,
  downloadExportTask,
  deleteExportTask,
  t,
}) => {
  const showSkeleton = useMinimumLoadingTime(loadingStat);
  const needSkeleton = !showStat || showSkeleton;
  const [exportFormat, setExportFormat] = useState('csv');
  const [exportCenterOpen, setExportCenterOpen] = useState(false);

  useEffect(() => {
    if (!exportCenterOpen) return undefined;
    loadExportTasks();
    const timer = window.setInterval(loadExportTasks, 3000);
    return () => window.clearInterval(timer);
  }, [exportCenterOpen, loadExportTasks]);

  const placeholder = (
    <Space>
      <Skeleton.Title style={{ width: 108, height: 21, borderRadius: 6 }} />
      <Skeleton.Title style={{ width: 65, height: 21, borderRadius: 6 }} />
      <Skeleton.Title style={{ width: 64, height: 21, borderRadius: 6 }} />
    </Space>
  );

  return (
    <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
      <Skeleton loading={needSkeleton} active placeholder={placeholder}>
        <Space>
          <Tag
            color='blue'
            style={{
              fontWeight: 500,
              boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
              padding: 13,
            }}
            className='!rounded-lg'
          >
            {t(textQuotaUsed)}: {renderQuota(stat.quota)}
          </Tag>
          <Tag
            color='pink'
            style={{
              fontWeight: 500,
              boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
              padding: 13,
            }}
            className='!rounded-lg'
          >
            RPM: {stat.rpm}
          </Tag>
          <Tag
            color='white'
            style={{
              border: 'none',
              boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
              fontWeight: 500,
              padding: 13,
            }}
            className='!rounded-lg'
          >
            TPM: {stat.tpm}
          </Tag>
        </Space>
      </Skeleton>

      <Space>
        <Button
          size='small'
          theme='outline'
          loading={exportingFormat === 'detail:' + exportFormat}
          disabled={exportingFormat !== null}
          onClick={() => exportBillingLogs(exportFormat, 'detail')}
        >
          {t(textExportDetails)}
        </Button>
        <Button
          size='small'
          theme='outline'
          loading={exportingFormat === 'reconciliation:' + exportFormat}
          disabled={exportingFormat !== null}
          onClick={() => exportBillingLogs(exportFormat, 'reconciliation')}
        >
          {t(textExportReconciliation)}
        </Button>
        <Select
          size='small'
          value={exportFormat}
          onChange={setExportFormat}
          disabled={exportingFormat !== null}
          style={{ width: 86 }}
          optionList={[
            { value: 'csv', label: 'CSV' },
            { value: 'txt', label: 'TXT' },
          ]}
          aria-label={t(textFileType)}
        />
        <Button
          size='small'
          theme='borderless'
          onClick={() => setExportCenterOpen(true)}
        >
          {t(textDownloadCenter)}
        </Button>
        <Modal
          title={t(textDownloadCenter)}
          visible={exportCenterOpen}
          onCancel={() => setExportCenterOpen(false)}
          footer={null}
          width={720}
          bodyStyle={{ paddingBottom: 20 }}
        >
          <div className='flex justify-end mb-3'>
            <Button
              size='small'
              theme='outline'
              loading={loadingExportTasks}
              onClick={loadExportTasks}
            >
              {t(textRefresh)}
            </Button>
          </div>
          <div className='space-y-3 max-h-[420px] overflow-auto pr-1 pb-4'>
            {exportTasks.length === 0 ? (
              <div className='text-center text-gray-500 py-8'>
                {t(textNoExportTasks)}
              </div>
            ) : (
              exportTasks.map((task) => (
                <div
                  key={task.id}
                  className='border border-gray-200 rounded-md px-3 py-3 space-y-2 bg-white'
                >
                  <div className='flex flex-col sm:flex-row sm:items-start sm:justify-between gap-2'>
                    <div className='min-w-0 space-y-1'>
                      <Space wrap>
                        <Tag color={statusColor(task.status)}>
                          {t(task.status)}
                        </Tag>
                        <span className='font-medium'>
                          {task.kind === 'reconciliation'
                            ? t(textExportReconciliation)
                            : t(textExportDetails)}
                        </span>
                        <span className='text-xs text-gray-500 uppercase'>
                          {task.format}
                        </span>
                      </Space>
                    </div>
                    <Space spacing={6} className='shrink-0'>
                      <Button
                        size='small'
                        theme='outline'
                        disabled={task.status !== 'success'}
                        onClick={() => downloadExportTask(task)}
                      >
                        {t(textDownload)}
                      </Button>
                      <Button
                        size='small'
                        theme='outline'
                        type='danger'
                        disabled={
                          task.status === 'pending' || task.status === 'running'
                        }
                        onClick={() => deleteExportTask(task)}
                      >
                        {t(textDelete)}
                      </Button>
                    </Space>
                  </div>
                  <Progress percent={task.progress || 0} size='small' />
                  <div className='text-xs text-gray-500 flex flex-wrap gap-x-4 gap-y-1 leading-5'>
                    <span>
                      {t(textRows)}: {task.rows}
                    </span>
                    <span>
                      {t(textSize)}: {formatExportSize(task.file_size)}
                    </span>
                    <span>
                      {t(textCreatedAt)}: {formatExportTime(task.created_at)}
                    </span>
                    {task.error && (
                      <span className='text-red-500 break-all'>{task.error}</span>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
        </Modal>
        <CompactModeToggle
          compactMode={compactMode}
          setCompactMode={setCompactMode}
          t={t}
        />
      </Space>
    </div>
  );
};

export default LogsActions;
