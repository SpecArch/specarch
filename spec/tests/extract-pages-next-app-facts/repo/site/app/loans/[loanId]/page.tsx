export async function generateStaticParams() {
  return [{ loanId: 'l-1' }, { loanId: 'l-2' }];
}

export default function LoanPage() {
  return <h1>Loan</h1>;
}
