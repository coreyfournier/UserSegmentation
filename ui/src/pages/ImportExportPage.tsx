import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '../api/client';
import type { Snapshot } from '../api/types';
import ErrorBanner from '../components/common/ErrorBanner';
import styles from './ImportExportPage.module.css';

export default function ImportExportPage() {
  const qc = useQueryClient();
  const [jsonText, setJsonText] = useState('');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [loading, setLoading] = useState(false);

  const handleExport = async () => {
    try {
      const snap = await apiFetch<Snapshot>('/v1/admin/export');
      setJsonText(JSON.stringify(snap, null, 2));
      setError('');
      setSuccess('Config exported successfully');
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const handleImport = async () => {
    setError('');
    setSuccess('');
    setLoading(true);
    try {
      const snap = JSON.parse(jsonText);
      await apiFetch<Snapshot>('/v1/admin/import', {
        method: 'POST',
        body: JSON.stringify(snap),
      });
      qc.invalidateQueries({ queryKey: ['layers'] });
      setSuccess('Config imported successfully');
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const handleDownload = () => {
    const blob = new Blob([jsonText], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'segments.json';
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className={styles.page}>
      <h2>Import / Export</h2>

      {error && <ErrorBanner message={error} />}
      {success && <div className={styles.success}>{success}</div>}

      <div className={styles.actions}>
        <button className="btn-primary" onClick={handleExport}>Export Current Config</button>
        {jsonText && <button className="btn-ghost" onClick={handleDownload}>Download JSON</button>}
      </div>

      <div className="form-group">
        <label>Config JSON</label>
        <textarea
          value={jsonText}
          onChange={(e) => setJsonText(e.target.value)}
          rows={20}
          placeholder="Paste or export config JSON here..."
        />
      </div>

      <button
        className="btn-primary"
        onClick={handleImport}
        disabled={!jsonText || loading}
      >
        {loading ? 'Importing...' : 'Import Config'}
      </button>
    </div>
  );
}
