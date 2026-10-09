import { Field, Form, Formik } from 'formik';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

export const VisitorPage = () => {
  const { t } = useTranslation();
  const heading = t('visitor.title');
  return (
    <Formik initialValues={{ email: '', phone: '' }} onSubmit={() => {}}>
      <Form>
        <h1>{heading}</h1>
        <Field name="email" type="email" />
        <Field name="phone" />
        <Link to="/visits">{t('back')}</Link>
        <Link to="/visits/new">{t(`visits.${'new'}`)}</Link>
      </Form>
    </Formik>
  );
};
