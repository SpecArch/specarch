import { useForm } from 'react-hook-form';
import { useNavigate } from 'react-router-dom';

interface VisitForm {
  name: string;
  host: string;
  arrives: string;
}

export default function NewVisitPage() {
  const { register, handleSubmit } = useForm<VisitForm>();
  const navigate = useNavigate();
  const field = 'badge';
  return (
    <form onSubmit={handleSubmit(() => navigate('/visits'))}>
      <h1>Register a visit</h1>
      <input {...register('name')} />
      <input {...register('host')} />
      <input type="datetime-local" {...register('arrives')} />
      <input {...register(field)} />
    </form>
  );
}
