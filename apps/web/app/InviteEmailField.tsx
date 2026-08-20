"use client";

type OrgMemberOption = {
  sub: string;
  role: string;
  email?: string;
  displayName?: string;
};

export function InviteEmailField({
  listId,
  members,
}: {
  listId: string;
  members: OrgMemberOption[];
}) {
  const withEmail = members.filter((m) => (m.email || "").trim().length > 0);
  return (
    <>
      <input
        name="invitedEmail"
        type="email"
        list={withEmail.length > 0 ? listId : undefined}
        placeholder={withEmail.length > 0 ? "招待先メール（組織メンバーから選択可）" : "招待先メール（任意）"}
        style={{ width: 260 }}
        autoComplete="off"
      />
      {withEmail.length > 0 ? (
        <datalist id={listId}>
          {withEmail.map((m) => (
            <option key={m.sub} value={m.email}>
              {(m.displayName || m.sub) + (m.role ? ` · ${m.role}` : "")}
            </option>
          ))}
        </datalist>
      ) : null}
    </>
  );
}
