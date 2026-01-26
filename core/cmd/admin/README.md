# Atlas Admin CLI

Easy-to-run command-line tool for managing admin accounts.

## Commands

### Create New Admin
```powershell
cd e:\Development\Software\atlas\core
go run cmd/admin/main.go -action=create
```

You'll be prompted to enter:
- Admin Username
- Admin Password

### Reset Admin Password
```powershell
cd e:\Development\Software\atlas\core
go run cmd/admin/main.go -action=reset-password
```

You'll be prompted to enter:
- Username to Reset
- New Password

## Examples

**Creating your first admin:**
```powershell
go run cmd/admin/main.go -action=create
# Enter Admin Username: admin
# Enter Admin Password: SecurePassword123
# Successfully created admin: admin
```

**Resetting a forgotten password:**
```powershell
go run cmd/admin/main.go -action=reset-password
# Enter Username to Reset: admin
# Enter New Password: NewPassword456
# Successfully reset password for admin
```

## Notes
- The tool connects to your configured database automatically
- Passwords are hashed using bcrypt before storage
- You can create multiple admin accounts if needed
