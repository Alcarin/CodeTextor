<?php

namespace App\Services;

use App\Models\User;
use App\Interfaces\LoggerInterface;

/**
 * Class AuthService
 * Handles user authentication.
 */
class AuthService implements LoggerInterface {
    public const MAX_ATTEMPTS = 5;
    private $user;

    public function __construct(User $user) {
        $this->user = $user;
    }

    /**
     * Authenticate the user.
     * @param string $password
     * @return bool
     */
    public function login(string $password): bool {
        return $this->user->verifyPassword($password);
    }

    protected function log(string $message) {
        echo $message;
    }
}

interface LoggerInterface {
    public function log(string $message);
}

function global_helper() {
    return "I am a global function";
}

trait HelperTrait {
    public function traitMethod() {
        return "I am from a trait";
    }
}
