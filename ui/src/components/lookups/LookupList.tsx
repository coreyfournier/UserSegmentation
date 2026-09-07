import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
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

  // /lookups?edit=<id> opens that table's editor directly, so a lookup link on
  // a schema lands on the table rather than on the list. Same derive-at-render
  // shape as LayerList: an explicit click wins once set, and closing clears
  // both. Matching on id, which is what the link carries and what is stable.
  const [searchParams, setSearchParams] = useSearchParams();
  const editParam = searchParams.get('edit');
  const editingFromQuery = editParam ? lookups?.find((t) => t.id === editParam) ?? null : null;
  const activeEditing = editing ?? editingFromQuery;
  const closeEditModal = () => {
    setEditing(null);
    if (searchParams.has('edit')) {
      const next = new URLSearchParams(searchParams);
      next.delete('edit');
      setSearchParams(next, { replace: true });
    }
  };

  if (isLoading) return <p>Loading...</p>;
  if (error) return <ErrorBanner message={(error as Error).message} />;

  const tables = lookups ?? [];

  return (
    <div>
      <div className={styles.toolbar}>
        <h2>Lookup Tables</h2>
        <button type="button" className="btn-primary" onClick={() => setShowCreate(true)}>+ Add Lookup</button>
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
            <button type="button" className="btn-ghost btn-sm" onClick={() => setEditing(t)}>Edit</button>
            <button type="button" className="btn-danger btn-sm" onClick={() => setDeleting(t.id)}>Delete</button>
          </div>
        </div>
      ))}

      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Add Lookup Table">
        <LookupForm
          onSubmit={(table) =>
            createLookup.mutate(
              {
                name: table.name,
                keyType: table.keyType,
                description: table.description,
                emitOrder: table.emitOrder,
                customOrder: table.customOrder,
                entries: table.entries,
              },
              { onSuccess: () => setShowCreate(false) }
            )
          }
          onCancel={() => setShowCreate(false)}
        />
      </Modal>

      <Modal open={!!activeEditing} onClose={closeEditModal} title="Edit Lookup Table">
        {activeEditing && (
          <LookupForm
            initial={activeEditing}
            submitLabel="Save"
            onSubmit={(table) =>
              updateLookup.mutate(
                {
                  id: activeEditing.id,
                  table: {
                    ...activeEditing,
                    name: table.name,
                    description: table.description,
                    emitOrder: table.emitOrder,
                    customOrder: table.customOrder,
                    entries: table.entries,
                  },
                },
                { onSuccess: closeEditModal }
              )
            }
            onCancel={closeEditModal}
          />
        )}
      </Modal>

      <ConfirmDialog
        open={!!deleting}
        title="Delete Lookup Table"
        message={`Delete lookup "${deleting}"? This is blocked if any rule or output schema field references it.`}
        onConfirm={() => {
          if (deleting) deleteLookup.mutate(deleting);
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
    </div>
  );
}
