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

      {/* A table rather than a row of cards. These are all the same shape —
          name, id, key type, size, two ordering flags — so the interesting
          question is how they compare, and a column answers that at a glance
          where a card makes it a reading exercise.

          The ordering flags are shown, not edited: turning one on rearranges
          what the response carries, and customOrder also decides whether the
          entry list has authored numbers at all, so both belong beside the
          entries they govern rather than behind a stray click here. */}
      {tables.length > 0 && (
        <table className={styles.table}>
          <thead>
            <tr>
              <th>Name</th>
              <th>Stable id</th>
              <th>Key type</th>
              <th className={styles.numeric}>Entries</th>
              <th>Emits order</th>
              <th>Order from</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {tables.map((t) => (
              <tr key={t.id}>
                <td>
                  <button
                    type="button"
                    className={styles.nameLink}
                    onClick={() => setEditing(t)}
                    title={t.description || undefined}
                  >
                    {t.name}
                  </button>
                  {t.description && <div className={styles.desc}>{t.description}</div>}
                </td>
                <td><code className={styles.id}>{t.id}</code></td>
                <td><span className={styles.badge}>{t.keyType}</span></td>
                <td className={styles.numeric}>{t.entries?.length ?? 0}</td>
                <td>
                  {t.emitOrder ? 'yes' : <span className={styles.muted}>no</span>}
                </td>
                <td>
                  {/* Only meaningful once the order is emitted; saying so beats
                      showing a value that decides nothing. */}
                  {t.emitOrder
                    ? t.customOrder ? 'authored numbers' : 'list position'
                    : <span className={styles.muted}>—</span>}
                </td>
                <td className={styles.actions}>
                  <button type="button" className="btn-ghost btn-sm" onClick={() => setEditing(t)}>Edit</button>
                  <button type="button" className="btn-danger btn-sm" onClick={() => setDeleting(t.id)}>Delete</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}


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
