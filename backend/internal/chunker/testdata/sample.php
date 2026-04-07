<?php

namespace App\Services;

use App\Models\User;
use App\Contracts\AuthInterface;

// TODO: implement token refresh

/**
 * Handles user authentication.
 */
class AuthService implements AuthInterface
{
    const TOKEN_EXPIRY = 3600;

    public function login(string $email, string $password): bool
    {
        $user = User::findByEmail($email);
        return password_verify($password, $user->password);
    }

    protected function generateToken(User $user): string
    {
        // FIXME: use proper JWT
        return base64_encode($user->id . ':' . time());
    }

    private function validateSession(): bool
    {
        return session_status() === PHP_SESSION_ACTIVE;
    }
}

interface Loggable
{
    public function log(string $message): void;
}

trait Timestampable
{
    public function getCreatedAt(): string
    {
        return $this->created_at;
    }
}

function formatDate(string $date): string
{
    return date('Y-m-d', strtotime($date));
}

$result = formatDate('2025-01-01');
?>
