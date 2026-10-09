import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

export function VisitsPage() {
  const { t } = useTranslation();
  return (
    <main>
      <h1>{t('visits.title')}</h1>
      <Link to="/visits/new">{t('visits.new')}</Link>
      <Link to={`/visitors/${'v1'}`}>Last visitor</Link>
      <Link to="/help">{t('help.link')}</Link>
    </main>
  );
}
