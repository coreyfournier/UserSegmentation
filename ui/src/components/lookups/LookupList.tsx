import { useState } from 'react';
import { useLookups, useCreateLookup, useUpdateLookup, useDeleteLookup } from '../../api/lookups';
import type { LookupTable } from '../../api/types';
import Modal from '../common/Modal';
import ConfirmDialog from '../common/ConfirmDialog';
import ErrorBanner from '../common/ErrorBanner';
import LookupForm from './LookupForm';
import styles from './LookupList.module.css';

export default function LookupList() {
  const { data: lookups, isLoading, error } = useLookups();
  const createLookup = useCreateLookup();
  const updateLookup = useUpdateLookup();
  const deleteLookup = useDeleteLookup();

  const [showCreate, setShowCreate] = useState(false);
  const [editing, setEditing] = useState<LookupTable | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);

  if (isLoading) return <p>Loading...</p>;
  if (error) return <ErrorBanner message={(error as Error).message} />;

  const tables = lookups ?? [];

  return (
    <div>
      <div className={styles.toolbar}>
        <h2>Lookup Tables</h2>
        <button className="btn-primary" onClick={() => setShowCreate(true)}>+ Add Lookup</button>
      </div>

      {createLookup.error && <ErrorBanner message={(createLookup.error as Error).message} />}
      {updateLookup.error && <ErrorBanner message={(updateLookup.error as Error).message} />}
      {deleteLookup.error && <ErrorBanner message={(deleteLookup.error as Error).message} />}

      {tables.length === 0 && <p style={{ color: 'var(--text-muted)' }}>No lookup tables yet.</p>}

      {tables.map((t) => (
        <div key={t.id} className={styles.card}>
          <div className={styles.info}>
            <span className={styles.name}>{t.name}</span>
            <code className={styles.id}>{t.id}</code>
            <span className={styles.badge}>{t.keyType}</span>
            <span className={styles.count}>{t.entries?.length ?? 0} entries</span>
          </div>
          <div className={styles.actions}>
            <button className="btn-ghost btn-sm" onClick={() => setEditing(t)}>Edit</button>
            <button className="btn-danger btn-sm" onClick={() => setDeleting(t.id)}>Delete</button>
          </div>
        </div>
      ))}

      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Add Lookup Table">
        <LookupForm
          onSubmit={(table) =>
            createLookup.mutate(
              { name: table.name, keyType: table.keyType, entries: table.entries },
              { onSuccess: () => setShowCreate(false) }
            )
          }
          onCancel={() => setShowCreate(false)}
        />
      </Modal>

      <Modal open={!!editing} onClose={() => setEditing(null)} title="Edit Lookup Table">
        {editing && (
          <LookupForm
            initial={editing}
            submitLabel="Save"
            onSubmit={(table) =>
              updateLookup.mutate(
                { id: editing.id, table: { ...editing, name: table.name, entries: table.entries } },
                { onSuccess: () => setEditing(null) }
              )
            }
            onCancel={() => setEditing(null)}
          />
        )}
      </Modal>

      <ConfirmDialog
        open={!!deleting}
        title="Delete Lookup Table"
        message={`Delete lookup "${deleting}"? This is blocked if any rule references it.`}
        onConfirm={() => {
          if (deleting) deleteLookup.mutate(deleting);
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
    </div>
  );
}
