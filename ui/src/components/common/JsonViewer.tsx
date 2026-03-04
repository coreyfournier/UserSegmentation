import styles from './JsonViewer.module.css';

export default function JsonViewer({ data }: { data: unknown }) {
  return (
    <pre className={styles.pre}>
      {JSON.stringify(data, null, 2)}
    </pre>
  );
}
