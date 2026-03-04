import styles from './ErrorBanner.module.css';

export default function ErrorBanner({ message }: { message: string }) {
  if (!message) return null;
  return <div className={styles.banner}>{message}</div>;
}
