import { ScreenList } from "@acme/screens";
import { schema } from "./page.schema";

export default async function MemberLoansPage({ params }: { params: Promise<{ cardNumber: string }> }) {
  const { cardNumber } = await params;
  return <ScreenList schema={schema} params={{ cardNumber }} />;
}
